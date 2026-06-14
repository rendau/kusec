package kube

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/mechta-market/kusec/internal/errs"
)

const (
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "kusec"
	managedBySelector   = managedByLabelKey + "=" + managedByLabelValue

	appIdAnnotation       = "kusec.io/app-id"
	secretIdAnnotation    = "kusec.io/secret-id"
	configMapIdAnnotation = "kusec.io/configmap-id"
)

// Service синхронизирует секреты из базы в Kubernetes.
// Работает только изнутри кластера (in-cluster config + ServiceAccount).
type Service struct {
	appSvc        AppServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI

	mu sync.Mutex // один sync одновременно

	clientMu sync.Mutex
	client   kubernetes.Interface
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
) *Service {
	return &Service{
		appSvc:        appSvc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
	}
}

// getClient лениво создаёт in-cluster клиент. Успех кэшируется; ошибка — нет,
// чтобы временный сбой не залипал до рестарта.
func (s *Service) getClient() (kubernetes.Interface, error) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.client != nil {
		return s.client, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		if errors.Is(err, rest.ErrNotInCluster) {
			return nil, errs.NotInCluster
		}
		return nil, fmt.Errorf("rest.InClusterConfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes.NewForConfig: %w", err)
	}

	s.client = client
	return client, nil
}

// systemNamespaces — служебные namespace-ы Kubernetes, скрываемые из выбора.
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

// ListNamespaces возвращает имена namespace-ов кластера (без системных
// kube-*), отсортированные по алфавиту. Вне кластера это штатная ситуация:
// возвращается inCluster=false без ошибки.
func (s *Service) ListNamespaces(ctx context.Context) ([]string, bool, error) {
	client, err := s.getClient()
	if err != nil {
		if errors.Is(err, errs.NotInCluster) {
			return nil, false, nil
		}
		return nil, false, err
	}

	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("k8s: list namespaces: %w", err)
	}

	names := make([]string, 0, len(list.Items))
	for _, namespace := range list.Items {
		if systemNamespaces[namespace.Name] {
			continue
		}
		names = append(names, namespace.Name)
	}
	sort.Strings(names)

	return names, true, nil
}

// Sync выполняет общую синхронизацию: и секреты, и configmap-ы за один проход
// под единой блокировкой, чтобы между двумя видами объектов не было гонки и
// чтобы параллельный sync не стартовал в середине.
func (s *Service) Sync(ctx context.Context, appIds []string) (*SyncResult, *SyncResult, error) {
	if !s.mu.TryLock() {
		return nil, nil, errs.SyncInProgress
	}
	defer s.mu.Unlock()

	client, err := s.getClient()
	if err != nil {
		return nil, nil, err
	}

	secrets, err := s.syncSecretsLocked(ctx, client, appIds)
	if err != nil {
		return nil, nil, err
	}

	configMaps, err := s.syncConfigMapsLocked(ctx, client, appIds)
	if err != nil {
		return nil, nil, err
	}

	return secrets, configMaps, nil
}

func (s *Service) ensureNamespace(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	ensured map[string]bool,
) error {
	if ensured[namespace] {
		return nil
	}

	_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}, metav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(err) {
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("ensure namespace %q: %w", namespace, err)
	}

	ensured[namespace] = true
	return nil
}

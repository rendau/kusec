.DEFAULT_GOAL := build

BINARY_NAME = svc
BUILD_PATH = cmd/build
SERVICE_NAME = kusec_v1
ADMIN_PATH = apps/admin

# vendor repo sparse dirs
protos-dirs = google protoc-gen-openapiv2 common
vendor-proto-dirs = vp-common

.SILENT:

build:
	mkdir -p $(BUILD_PATH)
	CGO_ENABLED=0 go build -o $(BUILD_PATH)/$(BINARY_NAME) cmd/main.go

clean:
	rm -rf $(BUILD_PATH)

run-admin:
	pnpm --dir $(ADMIN_PATH) dev

# Локальный docker-образ (бэкенд + вшитая админка, сборка целиком внутри Docker).
docker-build:
	docker build -f deploy/docker/Dockerfile -t kusec:local .

lint:
	golangci-lint run

# Установка плагинов protoc
proto-plugins:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

generate-proto-$(SERVICE_NAME):
	mkdir -p pkg/proto
	protoc -I vendor-proto -I api/proto \
	--go_out pkg/proto --go_opt paths=source_relative \
	--go_opt=Mcommon/common.proto=`go list -m `/pkg/proto/common \
	--go-grpc_out pkg/proto --go-grpc_opt paths=source_relative \
	--grpc-gateway_out pkg/proto --grpc-gateway_opt paths=source_relative \
	--openapiv2_out=json_names_for_fields=false,allow_merge=true,merge_file_name=api:docs \
	api/proto/$(SERVICE_NAME)/*.proto

generate-proto: generate-proto-$(SERVICE_NAME)

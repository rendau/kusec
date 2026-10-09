# Логотип kusec

Знак — замок, корпус которого семиугольник (отсылка к форме логотипа Kubernetes):
kusec = **ku**bernetes + **sec**ret. Цвет — зелёный primary админки (`#18a058`),
в крупных размерах градиент `#3fbf7f → #0b6b3a`.

| Файл | Назначение |
|---|---|
| `favicon.svg` | иконка для таба и шапки, плоский цвет, пропорции усилены под 16–32 px |
| `mark.svg` | знак с градиентом для крупных размеров (страница входа, доки) |
| `logo.svg` | знак + надпись; цвет надписи сам подстраивается под светлую/тёмную тему ОС |
| `logo-light.svg`, `logo-dark.svg` | то же с фиксированным цветом надписи (README, презентации) |
| `apple-touch-icon.svg` | иконка «на экран» для iOS/Android: белый знак на зелёном фоне |

Все SVG строит `gen.py` (чистый Python, без зависимостей): `python3 brand/gen.py`.
Копии для приложения лежат в `apps/admin/public/` (`favicon.svg`, `favicon.png`, `mark.svg`,
`logo.svg`, `apple-touch-icon.png`); PNG отрисованы из SVG: 
`qlmanage -t -s 64 -o . favicon.svg`, `qlmanage -t -s 180 -o . apple-touch-icon.svg`.

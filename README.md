# PDF compression API — GS-v1

Внутрішній синхронний сервіс на Go 1.27.2, Ghostscript 10.08.0 і qpdf 12.4.2.
API працює без credentials; документи обробляються локально в тимчасових папках.
Stages 01–08 завершено для локального GS-v1 handoff: 96 quality cases та bounded
1/2-worker benchmark перевірено. Запуск на цільовому хості потребує перевірки
його runtime protections; відомі межі наведено в [operations](docs/operations.md).
Наступні виправлення policy/XMP/qpdf та перевірки поточної збірки:
[звіт](docs/policy-validation-fixes-report.md). Архів Stage 08 містить версію до цих виправлень.

## Запуск

```sh
cp .env.example .env
docker compose up --build -d
curl http://127.0.0.1:8080/readyz
curl --fail-with-body -D headers.txt -o compressed.pdf \
  -F file=@document.pdf -F profile=balanced http://127.0.0.1:8080/v1/compress
docker compose down
```

Compose типово прив'язує порт до `127.0.0.1`. Для доступу з внутрішньої мережі
задайте `BIND_IP` конкретною внутрішньою IP-адресою сервера. Максимум два запити
одночасно; третій отримує `503 worker_busy` та `Retry-After: 1` до приймання body.
На локальному snap Docker цього хоста `no-new-privileges` блокує запуск docker-init.
Для локальної перевірки тут використано `NO_NEW_PRIVILEGES=false`; типовий захист
у Compose залишається true. На хості, що підтримує цю опцію, залиште default.

Папка jobs належить одному екземпляру сервісу; не діліть її між серверами.
Runtime: непривілейований UID 65532, read-only root, writable tmpfs,
4 GiB RAM / 2 CPU / 128 PID загалом, init для reaping осиротілих процесів.

## Підтримка великих PDF і спеціальних структур

До 300 MiB та 1000 сторінок. `MAX_PAGES` можна зменшити; всі перевірки
використовують однакову максимальну межу. Із `ALLOW_SPECIAL_TYPES=true`
вкладення, tagged структура, форми, шари, анотації, JavaScript та multimedia
зберігаються через qpdf; Ghostscript для таких документів пропускається.
`balanced`/`compact` у цьому разі означають lossless fallback. Попередження
`*_preserved` описують збережені елементи; JavaScript не виконується і не видаляється.
Шифрування, цифрові підписи, PDF/A і Launch actions залишаються відхиленими.
Strict mode (`ALLOW_SPECIAL_TYPES=false`) залишає попередні відмови.

Lossless-only кандидат перевіряється за reachable catalog graph незалежно від
номерів об’єктів; змінювати можна stream encoding keys. Decoded embedded bytes
перевіряються SHA-256 (до 32 MiB на stream, 300 MiB сумарно); traversal caps
32 levels per direct object та 100000 nodes. Втрата структури повертає exact
original з `special_features_changed`. Tool/limit errors залишаються помилками.
Вхідний PDF із recoverable qpdf warnings повертається exact original без
спроб перезапису, із `source_had_stream_warnings,compression_skipped_source_warnings`.
Це не відновлення пошкоджених потоків і не підтвердження їх декодування.
Мінімальний відсоток економії не гарантується.

## HTTP

- `POST /v1/compress`: multipart, рівно одне поле `file`, необов'язкове `profile`.
  Порядок довільний; повтори/невідомі поля/порожній файл відхиляються.
- `auto`: qpdf JPEG, потім Ghostscript за недостатньої економії (див. нижче).
- `balanced` (default): 300 dpi; `compact`: 150 dpi; `lossless`: тільки qpdf.
- Успіх `200 application/pdf`, filename `compressed.pdf`, `Cache-Control: no-store`.
  `X-Compression-Status`: compressed/unchanged; `X-Optimization-Engine`: original/qpdf/ghostscript.
  `X-Original-Bytes`, `X-Output-Bytes`, `X-Saved-Percent` (2 decimal places),
  `X-Request-ID`, `X-Compression-Warnings`, `X-Input-Image-Objects`, `X-Output-Image-Objects`.
  Image counts — унікальні reachable XObjects; inline images/повтори/реальні зміни gs не рахуються.
- `unchanged` повертає exact original bytes; результат не більший за оригінал.
  Fallback може мати warnings; обіцянки відсотка економії немає.
- Помилки: JSON `{code,message,request_id}`; 400 multipart/profile, 413 bytes,
  415 non-PDF, 422 invalid/special/limits, 503 busy/tools unavailable,
  504 timeout, 500 storage/required validation. Сирі tool diagnostics не повертаються.
- `GET /healthz`: процес живий. `GET /readyz`: pinned версії tools, worker executable,
  writable temp. `ENABLE_LOSSY=false` потребує тільки qpdf і приймає тільки явно заданий lossless.

Повний опис: [OpenAPI](docs/openapi.yaml).
Запуск із готового пакета, зупинка і rollback: [operations](docs/operations.md).
Приймальні докази: [acceptance matrix](docs/acceptance-evidence.md).

## Конфігурація сервера

| Env | Default | Межа/призначення |
|---|---|---|
| LISTEN_ADDR | :8080 | адреса в контейнері |
| TEMP_DIR | /work/jobs | виділена папка, очищення тільки власних marked pdfjob-* |
| WORKER_PATH | /usr/local/bin/pdfworker | довірений серверний binary |
| QPDF_PATH / GS_PATH | qpdf / gs | довірені tools |
| MAX_FILE_BYTES | 314572800 | до 300 MiB |
| MAX_REQUEST_BYTES | 325058560 | до 310 MiB; не менше file limit |
| MAX_JOB_BYTES | 2147483648 | до 2 GiB; watcher кожні 20 ms та final check |
| MAX_WORKERS | 2 | можна 1 або 2 |
| MAX_PAGES | 1000 | можна зменшити |
| PROCESSING_TIMEOUT | 120s | від завершення upload, спільний для pipeline |
| READ_HEADER_TIMEOUT / READ_TIMEOUT / WRITE_TIMEOUT | 10s / 60s / 180s | write deadline поновлюється після upload |
| ALLOW_SPECIAL_TYPES | true | special structures use qpdf only; false keeps strict refusals |
| ENABLE_LOSSY | true | false відключає auto/balanced/compact |

Фіксовані analysis caps: 100000 objects, depth 32, image 40 MP, JSON 64 MiB,
XMP 4 MiB, extracted text 32 MiB, multipart text 1024 bytes.
Job directories 0700, files 0600; filename від клієнта не використовується як шлях.
Кожна обробка — окремий worker/process group; timeout, disconnect, shutdown
скасовують групу TERM → grace → KILL і чекають завершення перед cleanup.
Compose передає наведені ліміти, `ALLOW_SPECIAL_TYPES`, upload timeout 120s,
processing timeout 600s і write timeout 660s. Defaults самого binary для
processing/write лишаються 120s/180s; для великих PDF використовуйте Compose
або задайте правильні env явно (`PROCESSING_TIMEOUT`, не `JOB_TIMEOUT`).

Watcher допускає короткий overshoot між опитуваннями; сумарний hard storage bound
задає tmpfs Compose. `GOMEMLIMIT=384MiB` — soft GC limit worker, не окрема memory isolation.
Логи містять request ID, HTTP status/code, bytes/time/engine/warnings без вмісту PDF.

## Перевірки

Перед кожним запуском unit перевірте imports/helpers/init/config на DB interaction
та запишіть audit у `docs/test-dependency-audit.md`. Міграції не запускати.

```sh
make unit
docker build --target api-test -t pdf-compression:stage06-api-test .
docker run --rm --init --network none --read-only --cap-drop ALL \
  --memory 2g --cpus 2 --pids-limit 128 \
  --tmpfs /tmp:rw,exec,nosuid,size=1g pdf-compression:stage06-api-test
```

`exec` потрібний тільки тестовій папці: Go запускає тимчасові test binaries.
Production jobs і /tmp у Compose мають `noexec`. Реальні e2e працюють усередині
контейнера через loopback без зовнішньої мережі/БД. `make e2e` запускає цю ж harness.

[Notices](THIRD_PARTY_NOTICES.md), [stage tracker](docs/implementation-plans/pdf-compression-stages/README.md).
Runtime містить ліцензії, source archives, fonts/resources та checksums tools.

## Автоматичний профіль

Передайте `profile=auto` до `POST /v1/compress`. Чинний default `balanced`
та профілі `compact`/`lossless` залишаються без змін.

Auto створює перевірену lossless-копію, потім пробує qpdf
`--optimize-images --jpeg-quality=70`. Якщо найменший перевірений qpdf-результат
економить щонайменше 20% від оригіналу, Ghostscript пропускається. Інакше GS
працює з оригіналом на 200 DPI, і вибирається найменший валідний результат
серед усіх кандидатів та оригіналу. Це евристика швидкості, не пошук
глобально найменшого PDF. Кількість картинок не визначає вибір інструмента.

Налаштування сервера/Compose:

| Змінна | Default | Допустимі значення |
|---|---|---|
| AUTO_JPEG_QUALITY | 70 | 0–100 |
| AUTO_MIN_SAVED_PERCENT | 20 | 0–100, ціле число |
| AUTO_DPI | 200 | 150 або 200 |

DPI стосується кольорових і сірих зображень; монохромні не зменшуються.
200 DPI використовує параметри якості balanced, 150 DPI — compact.
Downsampling починається, якщо фактичний DPI зображення перевищує цільовий
у 1.1 раза; менші зображення не збільшуються.
Special/lossless-only та зображення з непридатними кольорами/фільтрами або
перевищенням ліміту пікселів зберігають чинний lossless fallback. Вхідні stream
warnings повертають exact original після успішного аналізу та policy/size checks. `ENABLE_LOSSY=false` відхиляє `auto`.
Помилка/попередження qpdf JPEG або невдала перевірка відкидає тільки JPEG-кандидат;
невдалий GS відкидається за чинними правилами. Timeout/cancel не маскуються.
Структурна/текстова перевірка не гарантує читабельність тексту всередині сканів:
для нового набору документів потрібен візуальний контроль.

```sh
curl -F 'file=@document.pdf' -F 'profile=auto' http://127.0.0.1:8080/v1/compress -o compressed.pdf
```

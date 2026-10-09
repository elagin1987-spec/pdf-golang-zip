# План реалізації HTTP-сервісу стиснення PDF

- Статус: **Completed for local handoff** — stages 01–08 виконано; цільовий host acceptance описано в operations.
- Дата / редакція: 2026-10-09 / **GS-v1**.
- Проєкт: `pdf-golang-zip`.
- Цільовий виконавець: Gemini 3.8 Flash або інший агент розробки.
- Поточний стан: HTTP runtime, quality/load та фінальний handoff виконано. Фактичні докази — у tracker/results і docs/acceptance-evidence.md.

Внутрішній HTTP API на Go приймає PDF, аналізує його через qpdf, стискає через Ghostscript у режимах balanced/compact і повертає найменший перевірений результат. Lossless використовує лише qpdf. Усі tools локальні в Docker. Код сервісу закритий, API без авторизації. Цей документ зберігає контракт реалізованого GS-v1.

## 1. Узгоджений обсяг

| Параметр | Рішення |
|---|---|
| Мова / середовище | Go, Linux, Docker |
| Основний рушій | **Open-source Ghostscript (AGPL), без ключа активації**, обраний користувачем |
| Допоміжний рушій | qpdf для analysis, policy, lossless і structural checks |
| Інтерфейс | HTTP API, синхронний POST; без UI, токенів/авторизації |
| Доступ | Внутрішня мережа, один сервер |
| Профілі | balanced 300 dpi типово; lossless; compact 150 dpi |
| Вхід | Звичайні PDF і скани; спеціальні типи відхиляти |
| Дані | Тимчасові локальні файли; без БД/Redis/зовнішніх API |
| Робота | Вісім stages, кожен у своєму чаті з фактичним results-файлом |

Рішення взяти open-source Ghostscript є дозволом планувати та імплементувати цей стек; воно не є юридичним висновком щодо всіх умов AGPL конкретного використання/поширення. Зберегти AGPL текст/notices і походження binary у Docker та documentation. Комерційна ліцензія і ключі не закладаються. Раніше описаний Go-image-editor більше не є основною реалізацією.

Відсоток стиснення не гарантується. Якщо результат більший або не проходить перевірки, використати перевірений lossless-кандидат чи оригінал. Повернутий PDF ніколи не більший за оригінал.

## 2. Залежності

Runtime:

1. **Go 1.27.2**, переважно standard library: HTTP/multipart/JSON/process control/logging. Без x/image, власних JPEG/JPX/JBIG2 декодерів чи PDF content-stream lexer.
2. **Ghostscript 10.08.0 open-source**, CLI `gs`: pdfwrite для стиснення, txtwrite для runtime text comparison. Початкова version baseline відповідає перевіреній офіційній документації; Stage 01 перевіряє отримання саме цього release, фіксує source/artifact checksum і фактичне gs --version. Непомітно заміняти версію не можна.
3. **qpdf 12.4.2**, CLI: analysis/metadata, policy support, lossless structural optimization, validity checks. JSON version 2. Ліцензія Apache-2.0.

QA target: Poppler utilities `pdftoppm`, `pdftotext`, `pdfinfo`, окремо від production. Build dependencies обох CLI залишаються в build-stage. Lock versions/image digests/runtime libraries і notices після відтворюваної збірки. PDF processing не завантажує залежності й не викликає зовнішні сервіси.

[Ghostscript releases/licenses](https://ghostscript.com/releases/gsdnld.html), [pdfwrite](https://ghostscript.readthedocs.io/en/latest/VectorDevices.html), [Ghostscript usage](https://ghostscript.readthedocs.io/en/latest/Use.html), [qpdf](https://qpdf.readthedocs.io/en/stable/cli.html), [qpdf license](https://qpdf.readthedocs.io/en/stable/license.html). [Engine options](pdf-compression-engine-options.md) тепер є довідником альтернатив, а не основним дизайном.

## 3. Роль рушія і формати

Ghostscript інтерпретує вхід і створює новий PDF. Формати/кольори/маски декодує він, а не Go. У PDF визначати поточні filters/BPC/ColorSpace/Mask/SMask, не початкові назви PNG/TIFF. qpdf analysis збирає inventory для перевірок і resource limits, а не для ручного image replacement.

| Вміст | Політика GS-v1 |
|---|---|
| JPEG/DCT, Flate, LZW, RunLength, Indexed, raw RGB/Gray | Обробка Ghostscript; downsampling кольорових/сірих images і auto JPEG/Flate choice; підтвердження fixtures |
| JPX/JPEG 2000 | Ghostscript decoder; passthrough коли застосовний; за resize можливе інше output encoding. Не обіцяти JPX encoder |
| CCITT/JBIG2, 1-bit scans | Не downsample mono; використовувати lossless mono encoding. Не обіцяти збереження ідентичних stream bytes |
| Mask/SMask/прозорість, inline images, nested Forms | Обробка pdfwrite; перевіряти fixtures, не робити власний обхід операторів/ресайз масок |
| CMYK/ICC/Separation/DeviceN | У першій версії пропустити весь GS-pass з `complex_color_preserved`; залишити qpdf lossless/original. Розширення лише після окремого QA-доказу |
| Невідомий image filter або image понад pixel cap | Весь GS-pass пропустити, `unsupported_image_filter` / `image_pixel_limit` |

JPEG2000/JBIG2 вже не виключаються лише через відсутність Go-декодера. Кожен заявлений codec має мати дозволений positive fixture до release. Якщо конкретний decoder у зафіксованій збірці не працює, pipeline повертає fallback із `ghostscript_failed`; не стверджувати підтримку на підставі назви бібліотеки.

Не виконувати OCR, rasterization всіх сторінок, обрізання, примусову RGB/grayscale-конвертацію або ZIP-обгортку. Ghostscript сам може перебудувати fonts/resources; byte identity content streams і image IDs не є контрактом balanced/compact.

## 4. Профілі та параметри Ghostscript

DPI — ефективна роздільність при розміщенні на сторінці. Ghostscript відповідає за downsampling; Go не обчислює CTM/placement. До 300/150 dpi означає ціль для eligible color/gray images, а не гарантію рівно такого DPI у кожному object. Малі картинки не збільшувати. Shared image у різних розміщеннях обов'язково перевірити fixture; не припускати збереження одного object чи відсутності cloning.

Профілі з фіксованих server settings:

| Параметр | balanced | compact |
|---|---|---|
| ColorImageResolution / GrayImageResolution | 300 | 150 |
| DownsampleColorImages / DownsampleGrayImages | true | true |
| ColorImageDownsampleType / GrayImageDownsampleType | Bicubic | Bicubic |
| ColorImageDownsampleThreshold / GrayImageDownsampleThreshold | 1.1 | 1.1 |
| DownsampleMonoImages | false | false |
| AutoFilterColorImages / AutoFilterGrayImages | true | true |
| QFactor у Color/Gray ACS dictionaries | 0.4 | 0.76 |
| Color JPEG sampling | H/V [1 1 1 1] | H/V [2 1 1 2] |
| Gray JPEG sampling | H/V [1 1 1 1] | H/V [1 1 1 1] |

QFactor не є JPEG quality 0–100. Ці значення є базовою конфігурацією для реалізації; Stage 04/07 підтверджують якість, а зміну значень документують як калібрування профілю. Не користуватися неявними presets /screen, /ebook або /printer. `lossless` ніколи не викликає pdfwrite.

Спільні параметри: `-dBATCH`, `-dNOPAUSE`, `-dSAFER`, `-sDEVICE=pdfwrite`, `-dCompatibilityLevel=1.7`, `-dAutoRotatePages=/None`, `-sColorConversionStrategy=LeaveColorUnchanged`, `-dConvertCMYKImagesToRGB=false`, `-dPassThroughJPEGImages=true`, `-dPassThroughJPXImages=true`, `-dEmbedAllFonts=true`, `-dSubsetFonts=true`, `-dCompressFonts=true`, `-dCompressStreams=true`, `-dDetectDuplicateImages=true`, `-dFastWebView=false`, `-dPreserveAnnots=true`.

Довірений settings-код через `-c` задає `NeverEmbed []`, ACS dictionaries і `/PreserveAnnotTypes [/Link] def`; input передається після `-f`. Ніякі PostScript snippets не надходять із запиту. Якщо probe довів, що Link preservation не працює навіть із цими settings, runtime semantic check відкидає GS-кандидат і повертає fallback; не видаляти посилання мовчки. Passthrough є best-effort, а не гарантією збереження JPEG при downsampling/масках.

Не робити повторних lossy passes: balanced/compact створюється один раз від input.pdf. Deduplication включена; benchmark має зафіксувати її вартість на PDF із багатьма images. Якщо вона порушує ресурсний контракт, окремо зафіксувати зміну config і targeted QA, а не збільшувати timeout мовчки.

## 5. Конвеєр

```text
input.pdf → qpdf preflight / policy / inventory
          → qpdf lossless.pdf → structural validation
          → за profile та eligibility: gs pdfwrite від input.pdf
                                     → gs-candidate.pdf
                                     → qpdf structural candidate (без image recompression)
                                     → validation
          → найменший valid candidate або input.pdf
          → HTTP response → cleanup
```

Preflight: `qpdf --is-encrypted` (0 = encrypted, 2 потребує подальшого check), потім окремий `--check` (0 success, 2/3 refusal), потім `--json=2 --json-stream-data=none`. UseNumber, цикли/кількість/depth bounded, resource graph inspection без content-stream lexer. XMP отримувати bounded stream read. Не використовувати update-from-json/raw image replacement.

Lossless: qpdf `--compress-streams=y --decode-level=generalized --recompress-flate --compression-level=9 --object-streams=generate`. Не застосовувати optimize-images або linearize. Ті самі structural flags можна застосувати після pdfwrite; GS-result без додаткового qpdf також є кандидатом, щоб не втратити економію через finalization overhead.

Go запускає CLI через args array, задає fixed profiles/output paths, watchdog/deadline і обробляє exit codes. Будь-який failed GS-pass або втрата властивостей відкидає GS-кандидат із warning. Timeout/disconnect/втрата storage чи required qpdf validation — помилка, а не прихований успіх.

## 6. Policy і перевірка кандидатів

Відхиляти до переписування: encryption, signature fields/dictionaries, AcroForm/XFA, declared PDF/A у bounded XMP, attachments/portfolio, JS/Launch/RichMedia, tagged PDF/StructTreeRoot, optional-content layers. Дозволити лише прості Link annotations без appearance; інші — special types. Link actions URI та GoTo нормалізувати семантично; нетипові actions відхиляти. Не проводити PDF/A conformance validation, лише відмовляти позначеним документам.

Для кожного кандидата: qpdf check exit 0; те саме число сторінок; фізична геометрія MediaBox/CropBox/UserUnit/Rotate еквівалентна з tolerance 0.01 point; порядок/текст сторінок перевіряється extraction; bookmarks/links і їх destinations семантично еквівалентні незалежно від object IDs. Властивості порівнюються з оригіналом, не з іншим rewritten candidate.

Runtime text comparison: gs txtwrite/TextFormat=3 отримує текст оригіналу й GS-кандидатів. Нормалізувати лише CRLF/CR→LF, Unicode NFC не вводити, trim trailing spaces у рядках і terminal blank lines. Інші differences — `ghostscript_text_changed` і fallback. txtwrite output є plain text без гарантованих page markers: не вигадувати per-page hash; page-order перевіряється також QA fixture/renders. Text extraction failure/cap відкидає GS-кандидат із `text_validation_unavailable`. Навіть при відсутності оригінального тексту переконатися, що output порожній. Lossless не потребує запуску gs для image processing: структурний invariant доводять qpdf/test fixtures; за GS-ineligible document не запускати txtwrite навіть для lossless.

gs warnings/repair/font substitution diagnostics роблять GS-кандидат неприйнятним у MVP з `ghostscript_warning`; normal progress stdout не вважається stderr warning. Діагностику обмежити й не віддавати сирою клієнту. Source preflight залишається strict. Якщо неможливо отримати обов'язковий structural snapshot, повернути error. Якщо не спрацювала лише додаткова text/semantic перевірка GS-кандидата, можна повернути valid lossless/original із warning.

Порівняння decoded content-stream hashes не застосовується до GS-result: pdfwrite переписує operators і fonts. Повна visual/text fidelity не доводиться одним qpdf check або однаковим текстом. Незалежна Poppler QA у Stage 07 є release gate для кольору, тексту, векторів, masks, OCR, QR/barcode і shared placements.

При рівності розмірів обрати original; `unchanged` означає exact original bytes. Fallback warnings: `complex_color_preserved`, `unsupported_image_filter`, `image_pixel_limit`, `ghostscript_failed`, `ghostscript_warning`, `ghostscript_text_changed`, `ghostscript_features_changed`, `text_validation_unavailable`. Document inventory не означає точну кількість зображень, які gs змінив.

## 7. HTTP API

`POST /v1/compress`, `multipart/form-data`:

- Рівно одне поле `file`, один PDF.
- Необов'язкове поле `profile`: `balanced` за замовчуванням, або `lossless`/`compact`.
- Відхиляти повтори полів, невідомі поля, більше одного файла, порожній файл. Порядок multipart-полів довільний.
- Не довіряти extension/MIME; `%PDF-` у початкових 1024 байтах є попередньою ознакою, qpdf підтверджує формат.

Успіх: `200`, PDF binary, `Content-Type: application/pdf`, `Content-Disposition: attachment; filename="compressed.pdf"`, `Cache-Control: no-store`. Заголовки: `X-Request-ID`, `X-Compression-Status` (`compressed`/`unchanged`), `X-Original-Bytes`, `X-Output-Bytes`, `X-Saved-Percent` (два десяткові знаки), `X-Compression-Warnings` (унікальні машинні коди, без даних PDF, до 1024 bytes), `X-Optimization-Engine` (`original`, `qpdf`, `ghostscript`), `X-Input-Image-Objects`, `X-Output-Image-Objects`. Повернутий оригінал має статус unchanged навіть за warnings.

Помилки JSON: `{"code":"...","message":"...","request_id":"..."}`. `400` — multipart/profile/fields; `413` — bytes limit; `415` — не-PDF; `422` — invalid PDF / unsupported features / page/object limits; `503` — worker busy або необхідний PDF-tool unavailable; `504` — processing timeout; `500` — обов'язковий internal validation чи storage failure. Не повертати сирий stderr Ghostscript/qpdf. Кількості image objects означають різні reachable image XObjects за аналізом ресурсів; inline images і кількість появ на сторінках не включені. Не заявляти кількість реально оптимізованих картинок: Ghostscript може змінювати структуру й IDs.

`GET /healthz`: процес живий. `GET /readyz`: зафіксовані gs і qpdf доступні і temp directory writable; при ENABLE_LOSSY=false потрібен лише qpdf. На POST busy — `503` і `Retry-After: 1`, без черги та автоматичних retry. Client disconnect скасовує обробку. Відправлення binary починається після всіх перевірок. Окремих analyze endpoints, async jobs або REST storage API у MVP немає.

Авторизації та rate-limit middleware не додавати. Ліміт двох worker потрібний для коректного використання пам'яті, а не як авторизаційна політика.

## 8. Ліміти, тимчасові файли і процеси

| Налаштування | Початкове значення |
|---|---|
| `LISTEN_ADDR` | `:8080` усередині контейнера |
| `MAX_FILE_BYTES` | 50 MiB |
| `MAX_REQUEST_BYTES` | 51 MiB |
| `MAX_PAGES` | 500 |
| `MAX_WORKERS` | 2 |
| `PROCESSING_TIMEOUT` | 120 s від завершення upload, спільний для всіх етапів |
| HTTP header/read/write timeouts | 10 / 60 / 180 s; response write deadline reset після upload |
| Multipart text field | До 1024 bytes; не накопичувати весь body у RAM |
| `MAX_IMAGE_PIXELS` | 40 megapixels на аналізований image XObject, без overflow; inline payloads обмежуються process/resource limits |
| JSON analysis / XMP / extracted text | 64 MiB / 4 MiB / 32 MiB відповідно |
| Object count / traversal depth | 100000 / 32 |
| Tools stdout diagnostic / stderr | 64 KiB кожен; JSON/text/binary мають окремі bounded sinks |
| Worker process tree | Одна job-група процесів; при timeout/cancel зупинити всю групу |
| Docker runtime limits | 2 GiB RAM, 2 CPU, 128 PID для **всього контейнера**, не на worker |

Якщо analysis виявив image понад pixel limit, весь Ghostscript-pass пропускається з `image_pixel_limit`; перевірений lossless/original залишається доступним. Локально пропустити одну картинку всередині pdfwrite без власного редагування неможливо. Застосовувати це саме перед txtwrite/pdfwrite до такого файла. Загальні JSON/object/time limits завершують запит помилкою; неповний аналіз спеціальних типів не допускає успішної відповіді. Обмеження XMP завершує policy-check відмовою, а text cap означає недоступність перевірки GS-кандидата і fallback із warning.

Спільна temp-папка — `/work/jobs`. Кожна job має випадкове ім'я, права 0700, files 0600; original filename не використовувати як шлях. Стежити за сумарними розмірами власної job, ліміт 512 MiB; вихідні candidate-файли обмежувати watcher і cleanup, не допускати необмеженого запису gs/qpdf. Temp volume не є постійним сховищем документів. Job очищається після завершення streaming, помилки, disconnect і timeout. Startup прибирає тільки власні залишки за виділеним шляхом і не торкається сторонніх файлів.

Ресурсна межа Docker є загальною; `GOMEMLIMIT=384MiB` для pdfworker — додатковий soft GC limit, не hard memory sandbox. Не застосовувати низький RLIMIT_AS до Go: runtime резервує virtual memory. Не обіцяти ізоляцію пам'яті окремої job, якої цей MVP не має. Benchmark підтверджує практичність двох worker.

HTTP-server запускає свій `pdfworker` окремим процесом; усі gs/qpdf children успадковують його process group. Сигнали TERM → короткий grace → KILL, потім wait/reap. Server shutdown припиняє приймання, скасовує active jobs, чекає cleanup. `os/exec` отримує масив аргументів, без shell. Ніяких довільних шляхів, URL, PostScript settings або tools flags із запиту. qpdf default parser limits не вимикати; Ghostscript запускати з SAFER, не використовувати NOSAFER. Права SAFER для довірених job paths налаштувати в adapter і довести у probe. Будь-які permit-file аргументи генерує сервер.

Контейнер непривілейований, root filesystem read-only, tmp volume writable. Внутрішній port mapping в compose типово `127.0.0.1:8080:8080`; у README описати зміну на конкретну внутрішню IP-адресу сервера. Публічний ingress, TLS, login та secrets у цей функціональний MVP не включати. Обробка документів не викликає мережеві API.

## 9. Карта файлів і stages

Усі продуктні файли плануються як нові. Порядок та scope окремих чатів визначає [stage index](pdf-compression-stages/README.md).

| Stage | Головні файли / symbols |
|---|---|
| 01 | go.mod/go.sum, Dockerfile probe/QA targets, notices, cmd/engine-probe/main.go, docs/engine-evaluation.md, testdata/pdf, Makefile, audit script |
| 02 | internal/qpdf/{client,json,objects,streams}.go: IsEncrypted/Check/Analyze/ReadStream/WriteLossless; internal/ghostscript/client.go: Rewrite/ExtractText; internal/worker/runner_linux.go |
| 03 | internal/pdf/{analyzer,policy,inventory,snapshot}.go: AnalyzeDocument/CheckPolicy/AssessGSEligibility/BuildSnapshot; без CTM lexer |
| 04 | internal/ghostscript/{profiles,settings,diagnostics}.go: ProfileSettings/BuildArgs/ParseDiagnostics; trusted PostScript dictionaries |
| 05 | internal/compression/{service,profiles,result,validation}.go: Compress/ValidateCandidate/SelectSmallest; fixed candidates і semantic/text checks |
| 06 | internal/storage/temp.go, worker/protocol.go, cmd/pdfworker/main.go, config/config.go, httpapi/{router,compress,errors}.go, cmd/server/main.go, observability/logging.go, OpenAPI/compose/env/README |
| 07 | integration/e2e fixtures і docs/quality-report.md, docs/benchmark-report.md |
| 08 | README, operations, notices, final Docker/config/tooling consistency і acceptance evidence |

Не додавати internal/images, content_lexer.go, placement.go, JSON update generator або x/image. Runtime image містить Go server/pdfworker, gs, qpdf та їхні runtime resources/fonts; QA image додатково Poppler. На Stage 01 потрібно перевірити потрібні fonts і SAFER access до job paths.

## 10. Перевірки без БД

Ніколи не запускати міграції. Перед КОЖНИМ unit run прочитати tests/imports/helpers/init/TestMain/config/dependencies і переконатися у відсутності DB interaction/data writes. Аудит записати в docs/test-dependency-audit.md. Unit: fake tools, httptest, приватні temp. Integration: real gs/qpdf у QA-контейнері, дозволені synthetic/open fixtures, без external APIs.

| Тести | Обов'язкові сценарії |
|---|---|
| internal/qpdf/json_test.go, streams_test.go, client_test.go | JSON refs/UseNumber, encryption special exit codes, check 2/3, bounded read, structural flags |
| internal/ghostscript/client_test.go, profiles_test.go, settings_test.go, diagnostics_test.go | Exact args і fixed settings; balanced/compact/mono; no shell/user PS; passthrough; gs failure/warnings; txtwrite text cap |
| internal/pdf/policy_test.go, inventory_test.go, snapshot_test.go | Special refusals, unknown filters, pixel overflow/cap, complex colors, reachable image counts, semantic links/bookmarks/physical boxes |
| internal/compression/service_test.go, validation_test.go | lossless never pdfwrite; valid smallest/original; candidate rejection; text/features differences; warning fallback; timeout/cancel/mandatory failure |
| internal/httpapi/compress_test.go | multipart order/duplicates/limits, all profiles, PDF response/headers, errors/busy/disconnect, no credentials |
| internal/storage/temp_test.go, internal/worker/runner_linux_test.go | Permissions, limits, cleanup, child tree TERM/KILL/reap, shutdown/parallel jobs |
| tests/integration/compress_test.go | real gs rewrite/qpdf lossless/check; formats/masks/inline/Form/OCR; links/bookmarks preservation або documented fallback |
| tests/e2e/api_test.go | Docker upload/download, unchanged/fallback, two workers, overload, timeout/disconnect, temp cleanup |

Fixtures: text/кирилиця/embedded і unembedded fonts, JPEG 600/150 dpi, RGB/Gray Flate/predictors, Indexed, JPX, CCITT/JBIG2, masks/transparency, inline/Form, shared image в різних розміщеннях, UserUnit/rotation, OCR, diagrams/small text/QR/barcode, CMYK/ICC fallback, special-type negative і malformed PDF. Runtime metadata/payload IDs не доводять, що pixel appearance збережено.

Release QA:

- Lossless: independent Poppler renders збігаються по pixels, text/геометрія/links/bookmarks збережені.
- GS profiles: independent Poppler text та геометрія збігаються; всі сторінки в правильному порядку; small text/QR/barcode читаються; колір, transparency і вектори перевірені на renders/crops. Expected 300/150 DPI перевіряється на known-placement fixtures, no upscale — на 150-dpi fixture.
- Моно: без downsampling і без lossy substitutions; identical decoded pixels на mono crops у positive fixtures, source codec bytes не зобов'язані збігатися.
- GS output може змінювати Produce/ModDate/IDs та representation metadata; не обіцяти byte identity. Не навмисно видаляти source metadata; за критичною metadata-семантикою цей MVP не є архівним editor.
- Unsupported/failed GS: відповідає valid qpdf/original fallback із точним warning, а не falsely optimized result.
- Benchmark: 1/2 jobs, до 500 pages/50 MiB, gs dedup cost, total processing time, peak RAM/temp disk, savings і cleanup. Звіти docs/quality-report.md та docs/benchmark-report.md мають факти, Pass/Fail/Not run.

Makefile targets build/unit/integration/e2e/qa/probe з'являються під час implementation; exact commands документуються після їх створення. Audit script допомагає, але не замінює прочитання dependencies перед unit run. У цьому плануванні жодні тести/CLI processing не запускалися.

## 11. Дані, запуск і відкат

Без DB/migrations/backfill. Stage 01 probe → 02 adapters → 03 analysis → 04 GS profiles → 05 pipeline → 06 API/worker → 07 quality/load → 08 handoff. Production deploy не входить у поточний запит.

ENABLE_LOSSY=false відключає balanced/compact із 400 profile_disabled; lossless працює без pdfwrite. У звичайному режимі readiness вимагає gs і qpdf; при відключеному lossy — лише qpdf. Відкат: попередній image/config, temporary jobs можуть завершитися помилкою; persistent data відсутні.

## 12. Критерії прийняття

1. Docker із Go/gs/qpdf зафіксованих версій та notices; без ключа Ghostscript, без API auth. Доказ: Stage 01/06/08.
2. Три профілі; lossless не запускає pdfwrite; exact profile flags зафіксовані й тестовані. Доказ: Stage 02/04/05.
3. Known 600-dpi color/gray fixtures стають 300/150 dpi; 150-dpi не збільшується; shared usage/rotation/UserUnit не псують appearance. Доказ: Stage 04/07.
4. Текст/OCR, physical page geometry/order, vectors, links/bookmarks збережені у прийнятому GS-кандидаті або явний fallback; visual QA пройдено. Доказ: Stage 03/05/07.
5. Formats із заявленої матриці реально працюють у selected build; mono без lossy/downsampling; unsupported cases мають truthful warnings. Доказ: Stage 01/04/07.
6. Повернутий PDF valid і не більший; unchanged exact original; failed candidates не віддаються. Доказ: Stage 05/06/07.
7. Special documents відхилено до rewrite; strict input check; limits/cancel/cleanup/process group доведено. Доказ: Stage 02/03/06/07.
8. Audit перед unit runs виконано; DB/migrations відсутні; logs без document content. Доказ: audit і source/log review.
9. Вісім stage results містять фактичні докази; загальна готовність тільки після Stage 08. Не підміняти engine settings фактичним доказом якості.

## 13. Stages в окремих чатах

[Tracker і prompts](pdf-compression-stages/README.md). Кожен stage має залежності, scope, acceptance та results-файл. Поточні стани наведено у tracker; усі stages мають фактичні GS-v1 reports. Результати попереднього Go-resizer варіанта не вигадуються. Журнал decisions зберігає історію й явний перехід на GS-v1.

## 14. Передача виконавцю

Прочитати весь план і локальні інструкції, потім лише заданий stage та попередній results. Реалізувати один stage в одному чаті; записати commands/results/artifacts, audit, deviations і точний handoff; оновити власний tracker row. Не створювати чати/commits/deploy автоматично. Не запускати міграції. Не повертатися до Go-image replacement чи змінювати engine непомітно. Першим доказом є real container probe gs/qpdf.

Expected changed files: таблиця Stage 01–08, відповідні тести розділу 10, fixtures/notices/reports/tooling, operations та tracker/results. Немає images/CTM/update-from-json code, DB/auth/UI modules. Умовні файли — лише licenses/fonts/runtime-library notices реально включеного Docker-образу.

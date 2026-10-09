# Ghostscript та альтернативи для цього сервісу

Дата: 2026-10-09. **Поточне рішення: користувач обрав open-source Ghostscript; основний план — GS-v1 (Go + gs + qpdf).** Нижче збережено пояснення попереднього вибору та альтернатив; це не поточний implementation contract і не виконаний benchmark.

## Чому Ghostscript прибрано з базового варіанта

Користувач підтвердив закритий код та внутрішню мережу, але не відмовлявся від Ghostscript і не визначав бюджет ліцензії. Виключення було консервативним інженерним припущенням, а не встановленою юридичною забороною.

Ghostscript має AGPL і комерційну ліцензію. Artifex публікує позицію, що server-based інтеграція без виконання AGPL-вимог потребує комерційної ліцензії. Комерційний варіант допускає proprietary застосування на погоджених умовах. [Позиція Artifex](https://artifex.com/licensing), [варіанти Ghostscript](https://ghostscript.com/releases/gsdnld.html).

Водночас сам текст AGPL дозволяє запуск незміненої програми; network-вимога §13 стосується modified version, а independent aggregation відрізняється від єдиного derived work. Тому фраза «закритий код — Ghostscript заборонений» некоректна. Значення мають зміни до рушія, спосіб інтеграції, поширення образу й коло користувачів. Внутрішня мережа сама по собі не вирішує ці питання. Окремий CLI або контейнер також не є автоматичною гарантією, що власний код не охоплюється зобов'язаннями. Для вибору AGPL потрібна оцінка конкретного сценарію; альтернативний однозначний шлях — погоджена комерційна ліцензія. [Текст ліцензії в репозиторії Ghostscript](https://raw.githubusercontent.com/ArtifexSoftware/ghostpdl/master/doc/COPYING).

## Технічний компроміс

Ghostscript pdfwrite інтерпретує документ і створює новий PDF. Це зручний спосіб downsampling і перекодування картинок, але структура не залишається тотожною. Частина non-marking content може не зберегтися. Текст і вектори не обов'язково rasterize; твердження «Ghostscript завжди робить картинки з усіх сторінок» було б помилкою. Потрібна перевірка links/bookmarks/OCR/fonts/color на fixtures. [Офіційна документація pdfwrite](https://ghostscript.readthedocs.io/en/latest/VectorDevices.html).

Для звичайних PDF і сканів Ghostscript технічно залишається сильним кандидатом. Виключення складних спеціальних документів з MVP зменшує частину ризиків переписування. Компресія все одно залежить від джерела; вже оптимізовані картинки можуть не зменшитися.

## Варіанти

| Варіант | Додаткові компоненти | Практичний результат | Компроміс |
|---|---|---|---|
| Go + Ghostscript, комерційна ліцензія | Ghostscript, за потреби qpdf для lossless | Швидший старт з готовим downsampling; форматами керує рушій | Вартість/умови договору; PDF переписується; потрібні QA fixtures |
| Go + Ghostscript AGPL у належному сценарії | Незмінений або погоджено modified Ghostscript | Та сама функціональна обробка | Спочатку встановити ліцензійні зобов'язання конкретної інтеграції; не робити висновок лише з назви «внутрішній сервіс» |
| Go + qpdf + x/image — попередній базовий план | qpdf CLI, x/image | Точкова заміна зображень, structural lossless; власний код закритий за дотримання notices | Потрібні власні placement/DPI і codec policy; рідкісні формати зберігаються, а не ресайзяться |
| Go API + Python worker з pikepdf/Pillow | Python, qpdf/pikepdf, Pillow і фактичні codecs | Готові PDF object API і content-stream parser зменшують обсяг власного лексера | Другий runtime; tracking CTM/Forms та quality policy все одно потрібні; ліцензії codecs перевірити |
| Go + pdfcpu | Go library Apache-2.0, image codecs | Менше зовнішніх програм | Доведення потрібних image APIs/форматів та розміщення до вибору; optimize не дорівнює повній 300-dpi оптимізації |
| Go + комерційний PDF SDK | SDK і його runtime | Можлива готова комплексна оптимізація | Конкретні можливості/вартість підтверджуються trial і договором; SDK у цьому плані не обраний |

qpdf не робить ресемплінг самостійно; Go реалізує його. [qpdf optimization](https://qpdf.readthedocs.io/en/stable/cli.html#optimizing-file-size).

pikepdf є Python-wrapper над qpdf і має API для заміни картинок та content-stream parser; parser не обходить Forms автоматично. [pikepdf](https://pikepdf.readthedocs.io/en/latest/index.html), [content streams](https://pikepdf.readthedocs.io/en/latest/topics/content_streams.html). pikepdf має MPL-2.0: file-level copyleft допускає поєднання з proprietary кодом; умови поширення і модифікацій MPL-файлів залишаються. [Ліцензія pikepdf](https://raw.githubusercontent.com/pikepdf/pikepdf/main/LICENSE.txt), [Mozilla FAQ](https://www.mozilla.org/en-US/MPL/2.0/FAQ/). Pillow виконує raster resize/encoding; підтримка конкретного codec залежить від build. [Pillow](https://pillow.readthedocs.io/en/stable/about.html).

pdfcpu має Apache-2.0, validation/optimization та image operations, але це не підтверджений replacement для всього pipeline цього плану. [pdfcpu](https://github.com/pdfcpu/pdfcpu). MuPDF/PyMuPDF також пов'язані з AGPL/комерційним вибором, тому не є очевидною заміною для усунення саме цього питання. [Ліцензування Artifex](https://artifex.com/licensing).

## Порівняльні рекомендації — до вибору GS-v1

Якщо важлива відсутність комерційної PDF-ліцензії та другого runtime, залишити Go + qpdf. Ціна цього рішення — більше власної PDF-логіки й обмежений набір ресайзів у MVP.

Якщо найважливіше швидко отримати ширшу обробку сканів і різних image codecs, розглянути Ghostscript після рішення про ліцензію. Для цього сценарію відсутність складних форм/PDF-A у MVP робить його практичнішим, ніж для довільного універсального PDF editor.

Якщо Python-worker допустимий і комерційний SDK небажаний, pikepdf/Pillow заслуговує окремого proof: може зменшити обсяг ручного parsing порівняно з qpdf JSON у Go. Це пропозиція, не мовчазна зміна вже описаного стека.

Stage-план зараз описує обраний Go + open-source Ghostscript + qpdf. Комерційний варіант не закладено. Колишня Go-resizer реалізація є альтернативою, не другою паралельною реалізацією. Точні settings/validation/stages див. в основному плані GS-v1.

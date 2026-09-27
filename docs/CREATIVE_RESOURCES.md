# Creative Resources

Этот документ описывает функциональность задач #35–#39.

## Materials and Textures

Материалы и текстуры представлены единой библиотекой с типом ресурса:
- category;
- name;
- description;
- tags;
- prompt_fragment;
- optional preview.

Системные элементы доступны для выбора, но не изменяются и не удаляются пользователем. Пользовательские элементы поддерживают update/delete. Выбор записывается в human actions и provenance.

## Style DNA

Style profiles хранят версию и параметры визуальных предпочтений пользователя. Анализ MVP является описательным и строится на зафиксированных creative choices.

Style profile не копирует конкретного художника или внешнюю работу. Влияние на prompt выключено по умолчанию и должно быть явно включено пользователем; даже при включении endpoint возвращает только suggestion, автоматическая мутация prompt не выполняется.

## Asset DNA and Personal Visual Language

Asset DNA и Personal Visual Language агрегируют признаки из собственных project history и human actions.

Результаты содержат source lineage и uncertainty. Они не должны автоматически становиться частью нового prompt.

## Rights Registry and Do Not Use

Rights Registry связывает asset/reference с ownership, license, license source и verification state.

Verification states:
- verified;
- unverified;
- unknown;
- restricted.

Unknown не преобразуется автоматически в разрешённый статус.

Do Not Use constraints поддерживают artist, image, reference, motif, brand, composition и style на глобальном и project уровнях.

## Layered Project and Manual Edit

MVP хранит layered state поверх flat image:
- base/image/mask/manual_edit/adjustment/group layers;
- asset and iteration lineage;
- visibility, opacity and blend mode;
- manual edit records;
- source/mask/result assets;
- apply/reject decision;
- idempotency key.

Исходные assets не перезаписываются. Реальная brush/lasso selection и AI inpainting/erase workflow через InvokeAI реализуются отдельно в #70.

## API and audit

All mutation endpoints validate authentication/ownership and use parameterized SQL. Creative mutations write human action and provenance records.

Repeatable manual-edit creation requires Idempotency-Key.

OpenAPI is updated in backend/openapi.yaml.

## Architecture boundary

The application remains the owner of project state and provenance:

Vue -> Go REST -> application/domain -> PostgreSQL/Redis/object storage.

Image providers remain replaceable infrastructure adapters. InvokeAI is intentionally excluded from #35–#39 implementation and is tracked as #70.

CI note: backend lint uses the current official golangci-lint GitHub Action configuration.

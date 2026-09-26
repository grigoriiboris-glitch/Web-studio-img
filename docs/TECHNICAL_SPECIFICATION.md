# Web Studio IMG — Полное техническое задание

Версия: 1.0
Статус: базовая спецификация проекта
Репозиторий: https://github.com/oleg3190/Web-studio-img
Принцип: Create. Refine. Prove.

## 1. Назначение

Web Studio IMG — веб-платформа для создания изображений с помощью AI, прежде всего YandexART, с сохранением полного процесса творчества, человеческого вклада, версий, референсов, параметров генерации и проверок сходства.

Платформа не должна быть просто оболочкой над image-generation API. Основная ценность:
- сохранение творческого процесса;
- фиксация человеческих решений;
- неизменяемая история итераций;
- управление промптами, материалами, текстурами и референсами;
- анализ композиции и сходства;
- персональный Visual DNA;
- доказуемая история происхождения результата;
- экспорт воспроизводимого Creation Report;
- AI-ассистент с проверкой фактов и объяснимыми рекомендациями.

Система не должна обещать 100% уникальность, гарантированное авторское право, отсутствие любых аналогов или проверку всего интернета. Используются формулировки: «значимого совпадения в проверенном наборе источников не обнаружено», «результат анализа сходства», «человеческий вклад зафиксирован», «история создания сохранена», «источник/лицензия не подтверждены».

## 2. Цели

### Функциональные

Пользователь должен иметь возможность:
1. создать проект;
2. сформулировать идею;
3. загрузить собственный эскиз;
4. добавить референсы;
5. создать и редактировать промпт;
6. получить AI-подсказки;
7. сгенерировать варианты;
8. сравнить варианты;
9. выбрать вариант;
10. изменить композицию;
11. выбрать материал, текстуру и стиль;
12. повторно сгенерировать изображение;
13. вести несколько веток;
14. сохранить каждую итерацию;
15. выполнить анализ сходства;
16. выполнить анализ композиции;
17. получить рекомендации по изменению композиции;
18. выполнить ручную доработку;
19. зафиксировать человеческие действия;
20. проверить provenance;
21. экспортировать итог и доказательства процесса.

### Нефункциональные

Система должна быть расширяемой, безопасной, приватной по умолчанию, наблюдаемой, тестируемой, отказоустойчивой, пригодной к масштабированию и независимой от конкретного image provider.

## 3. Архитектурные принципы

1. Modular Monolith First. Микросервисы не использовать на старте.
2. Backend: Go.
3. Frontend: Vue 3 + TypeScript.
4. PostgreSQL — source of truth бизнес-данных.
5. Redis — очередь и кэш, но не source of truth.
6. Object Storage — изображения, маски, исходники и экспорт.
7. YandexART — инфраструктурный provider adapter.
8. Бизнес-логика не зависит от YandexART.
9. Все творческие операции фиксируются в provenance.
10. История итераций immutable.
11. Восстановление старой версии создаёт новую итерацию.
12. AI suggestions отделены от human decisions.
13. AI не имеет прямого SQL-доступа.
14. AI действует через ограниченные tools.
15. Любая AI-рекомендация объяснима и отменяема.
16. Проекты приватны по умолчанию.
17. Секреты не хранятся во frontend, Git или открытом конфиге.
18. После каждого изменения кода GitHub CI должен быть зелёным.

## 4. Технологический стек

### Backend
- Go
- REST API
- OpenAPI
- PostgreSQL
- sqlc
- Redis
- Asynq или эквивалентная очередь
- Yandex Object Storage (S3-compatible)
- MinIO для локальной разработки
- SSE для realtime
- OpenTelemetry
- Prometheus
- Grafana
- Docker

### Frontend
- Vue 3
- TypeScript
- Vite
- Pinia
- Vue Router
- Element Plus
- OpenAPI-generated API client
- SSE client

### Infrastructure
- Docker Compose
- GitHub Actions
- Yandex Object Storage (S3 API)
- PostgreSQL backups

## 5. Структура репозитория

/
├── backend/
│   ├── cmd/api/
│   ├── cmd/worker/
│   ├── internal/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── projects/
│   │   ├── iterations/
│   │   ├── generations/
│   │   ├── prompts/
│   │   ├── assets/
│   │   ├── references/
│   │   ├── materials/
│   │   ├── textures/
│   │   ├── styles/
│   │   ├── similarity/
│   │   ├── provenance/
│   │   ├── exports/
│   │   ├── providers/image/yandexart/
│   │   ├── storage/
│   │   ├── queue/
│   │   └── http/
│   ├── migrations/
│   ├── tests/
│   ├── Dockerfile
│   ├── go.mod
│   └── Makefile
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   ├── router/
│   │   ├── api/
│   │   ├── stores/
│   │   ├── components/
│   │   ├── layouts/
│   │   ├── features/
│   │   │   ├── projects/
│   │   │   ├── studio/
│   │   │   ├── generation/
│   │   │   ├── timeline/
│   │   │   ├── prompts/
│   │   │   ├── references/
│   │   │   ├── similarity/
│   │   │   ├── provenance/
│   │   │   ├── materials/
│   │   │   ├── textures/
│   │   │   └── styles/
│   │   ├── types/
│   │   ├── composables/
│   │   └── utils/
│   └── package.json
├── docs/
│   └── TECHNICAL_SPECIFICATION.md
├── .github/workflows/
├── docker-compose.yml
├── README.md
└── LICENSE

## 6. Backend architecture

Слои:
HTTP Handler → Application Service → Domain → Repository/Port → Infrastructure Adapter.

Provider-specific код запрещено использовать непосредственно из domain/application logic.

Основной интерфейс image provider:

type ImageGenerationProvider interface {
    Generate(ctx context.Context, request GenerateRequest) (GenerationJob, error)
    GetStatus(ctx context.Context, jobID string) (GenerationStatus, error)
    GetResult(ctx context.Context, jobID string) (GenerationResult, error)
}

В будущем допускаются ImageGenerationProvider, TextAIProvider, EmbeddingProvider, VisionProvider, SearchProvider и StorageProvider.

## 7. YandexART integration

YandexART изолировать в:
backend/internal/providers/image/yandexart/

Файлы:
- client.go
- provider.go
- mapper.go
- models.go
- errors.go

Бизнес-домен не должен знать API-модели YandexART.

Сохранять:
- provider;
- model;
- model_version;
- provider_job_id;
- prompt;
- negative_prompt;
- seed;
- aspect_ratio;
- generation parameters;
- request timestamp;
- completion timestamp;
- разрешённые provider metadata.

Не обещать детерминизм результата, если provider его не гарантирует.

## 8. Асинхронная генерация

POST /api/v1/projects/{project_id}/generations возвращает 202 Accepted и generation_id.

Pipeline:
API → Redis/Asynq → Go worker → YandexART → Object Storage/PostgreSQL → SSE.

Обязательны:
- Idempotency-Key;
- retry;
- exponential backoff;
- retryable/non-retryable errors;
- cancellation;
- timeout;
- quota checks;
- cost tracking.

## 9. Data model

### users
id, email, name, avatar, created_at, updated_at.

### projects
id, user_id, name, description, status, created_at, updated_at.

### iterations
id, project_id, parent_iteration_id, type, title, description, created_at.

Типы: idea, sketch, generation, selection, composition, prompt, manual_edit, final.

Iteration immutable.

### generations
id, project_id, iteration_id, provider, provider_job_id, model, model_version, prompt, negative_prompt, seed, aspect_ratio, parameters, status, error, estimated_cost, actual_cost, created_at, completed_at.

### assets
id, project_id, generation_id, type, storage_key, mime_type, size, width, height, sha256, metadata, created_at.

### prompts
id, project_id, iteration_id, parent_prompt_id, original_text, ai_suggestions, final_text, created_by, created_at.

created_by: human, ai, mixed.

### style_profiles
id, user_id, name, description, parameters, version, created_at.

### materials
id, name, category, tags, preview, description, prompt_fragment.

### textures
id, name, category, tags, preview, description, prompt_fragment.

### references
id, project_id, asset_id, source_url, source_type, license, license_verified, user_owned, notes, created_at.

source_type: inspiration, reference, direct_source, user_created, public_domain, unknown.

### human_actions
id, project_id, iteration_id, user_id, action_type, payload, created_at.

Примеры: IDEA_CREATED, PROMPT_EDITED, REFERENCE_ADDED, VARIANT_SELECTED, VARIANT_REJECTED, COMPOSITION_CHANGED, MATERIAL_SELECTED, TEXTURE_SELECTED, MANUAL_EDIT, APPROVED, EXPORT_CREATED.

### provenance_events
id, project_id, iteration_id, event_type, payload, parent_hash, hash, created_at.

Hash chain:
event[n].hash = SHA256(canonical(event[n].payload) + event[n].parent_hash)

### similarity_checks
id, project_id, asset_id, source, source_asset_id, visual_score, composition_score, semantic_score, style_score, metadata, created_at.

## 10. Creative Timeline

Timeline отображает:
Idea → Sketch → Prompt v1 → Generation → Variants → Selection → Composition change → Prompt v2 → Generation → Manual edit → Similarity check → Final.

Требования:
- все версии сохраняются;
- история не перезаписывается;
- поддерживаются branches;
- можно сравнивать branches;
- восстановление создаёт новую ветку;
- human и AI actions визуально различаются.

## 11. Human Contribution Map

Фиксировать:
- идея;
- исходный текст;
- изменения промпта;
- выбор варианта;
- композиционное решение;
- выбор материала;
- выбор текстуры;
- выбор референса;
- ручная обработка;
- отклонение AI-рекомендации;
- финальное утверждение.

Для каждого действия хранить actor, timestamp, version, old state, new state и AI influence при наличии.

Нельзя искусственно создавать человеческий вклад, которого пользователь не совершал.

## 12. Prompt Studio

Структурированные компоненты:
- subject;
- composition;
- camera;
- lighting;
- material;
- texture;
- color;
- atmosphere;
- style;
- depth;
- detail;
- constraints;
- negative constraints.

AI может предложить формулировку, улучшить структуру или найти конфликтующие параметры.

Пользователь может принять, отклонить или частично принять.

Хранить original human text, AI suggestions, final approved text, prompt version и parent prompt.

## 13. Material Library

Поддержка:
- категории;
- теги;
- поиск;
- preview;
- описание;
- prompt fragment;
- системные и пользовательские материалы.

Примеры: marble, brushed metal, raw wood, glass, ceramic, paper, textile, concrete, stone, liquid, plastic.

Выбор материала создаёт provenance event.

## 14. Texture Library

Поддержка:
- natural;
- geometric;
- organic;
- fabric;
- surface;
- abstract;
- custom.

## 15. Reference Management

Для каждого референса хранить:
- источник;
- URL;
- тип источника;
- license;
- license_verified;
- user_owned;
- hash;
- дату добавления;
- влияние на результат.

Если права неизвестны — состояние остаётся unknown.

## 16. Reference Influence Analysis

Для референса анализировать:
- composition;
- semantic content;
- color;
- style;
- material;
- geometry.

Результат содержит отдельные scores. При слишком близкой композиции показать предупреждение.

## 17. Composition Engine

Хранить:
- focal points;
- bounding boxes;
- relative positions;
- horizon;
- camera elevation;
- perspective;
- hierarchy;
- negative space;
- dominant geometry;
- object scale;
- light direction.

## 18. Composition Mutation Engine

При высоком композиционном сходстве предложить:
- изменить focal point;
- horizon;
- camera elevation;
- perspective;
- object scale;
- object placement;
- foreground obstruction;
- negative space;
- hierarchy;
- light direction;
- dominant geometry.

AI только предлагает изменения. Принятое изменение создаёт новую итерацию.

## 19. Similarity Engine

Разделить:
1. visual similarity;
2. composition similarity;
3. semantic similarity;
4. style similarity.

UI показывает каждый score отдельно.

Результат не является юридическим заключением.

Обязательно хранить:
- search scope;
- sources;
- timestamp;
- algorithm;
- algorithm version;
- недоступные источники.

Нельзя утверждать «проверено всё в интернете».

## 20. Similarity Search

MVP:
- perceptual hashes;
- image embeddings;
- metadata matching;
- composition descriptors.

Позже:
- pgvector;
- external search providers;
- custom vision models.

## 21. Style DNA

Style DNA — агрегированное описание реальных пользовательских предпочтений, не копирование конкретного художника.

Параметры:
- composition;
- lighting;
- texture;
- palette;
- geometry;
- perspective;
- depth;
- contrast;
- material;
- atmosphere.

Показывать recurring preferences, emerging preferences и differences from previous works.

Не применять Style DNA к новой работе без согласия пользователя.

## 22. Personal Visual Language

Анализировать собственные проекты пользователя и выявлять повторяющиеся паттерны. Выявленные паттерны не должны автоматически становиться частью prompt.

## 23. Asset DNA

Reusable assets:
- персонажи;
- объекты;
- символы;
- элементы композиции;
- текстуры;
- материалы.

Каждый asset имеет lineage и версии.

## 24. Explore / Develop / Finalize

Explore:
- быстрые генерации;
- много вариантов;
- низкая стоимость;
- широкий поиск.

Develop:
- меньше вариантов;
- больше детализации;
- работа с композицией, материалами, текстурами и референсами.

Finalize:
- фиксируются composition, prompt, materials, textures, references, model, parameters, seed, similarity checks и human approvals.

## 25. Layered Project

YandexART может возвращать плоское изображение. True layers реализуются платформой через masks, segmentation и manual edit layers.

MVP export:
- PNG/JPEG;
- masks;
- source assets;
- JSON manifest.

Позже:
- PSD;
- non-destructive editing;
- professional layered export.

## 26. AI Assistant

AI работает только через tools и не имеет прямого SQL-доступа.

Минимальные tools:
- create_iteration();
- get_project_history();
- analyze_composition();
- search_references();
- check_similarity();
- suggest_prompt();
- suggest_materials();
- create_generation();
- compare_iterations();
- verify_provenance().

AI должен объяснять рекомендации, показывать uncertainty, не выдавать неизвестное за факт, не менять данные молча и сохранять пользовательский контроль.

## 27. AI Fact Checker

Структура claim:
- claim;
- evidence;
- source;
- confidence;
- status.

Статусы:
- SUPPORTED;
- PARTIALLY_SUPPORTED;
- UNSUPPORTED;
- UNKNOWN.

Для юридических и иных high-stakes утверждений использовать первичные или официальные источники, когда доступны.

## 28. AI Recommendations

Каждая рекомендация содержит:
- recommendation;
- reason;
- evidence;
- confidence;
- affected entity;
- expected effect.

UI:
- Apply;
- Edit;
- Ignore.

Все действия записываются.

## 29. Rights Registry

Для внешнего asset/reference хранить:
- ownership;
- license;
- license source;
- verification state;
- verification date;
- notes.

Статусы:
- verified;
- unverified;
- unknown;
- restricted.

Не превращать unknown в разрешённое автоматически.

## 30. Do Not Use Constraints

Пользователь может запретить:
- конкретного художника;
- конкретное изображение;
- референс;
- мотив;
- бренд;
- композиционный шаблон;
- стиль.

Ограничения передаются AI tools и фиксируются в проекте.

## 31. Creation Report

Экспорт включает:
1. итоговое изображение;
2. timeline;
3. prompts;
4. generation parameters;
5. references;
6. human actions;
7. similarity checks;
8. provenance verification;
9. hashes;
10. manifest.

Форматы:
- PDF;
- provenance.json;
- manifest.json;
- hashes.txt;
- source files.

Creation Report описывает процесс и результаты проверок, но не является автоматически юридическим заключением.

## 32. Provenance verification

GET /api/v1/projects/{id}/provenance/verify

Проверять:
- event ordering;
- parent_hash;
- event hash;
- payload integrity;
- missing events;
- broken chain.

Пример ответа:
{
  "valid": true,
  "events_checked": 123,
  "broken_links": [],
  "verified_at": "..."
}

## 33. Audit Log vs Provenance

Provenance отвечает на вопрос «Как создавалось произведение?».

Audit Log отвечает на вопрос «Что происходило с аккаунтом и системой?».

Audit Log:
- login;
- logout;
- auth changes;
- permission changes;
- export;
- delete;
- API key actions;
- security events.

## 34. REST API

Base path: /api/v1

Projects:
GET /projects
POST /projects
GET /projects/{id}
PATCH /projects/{id}
DELETE /projects/{id}

Iterations:
GET /projects/{id}/iterations
POST /projects/{id}/iterations
GET /iterations/{id}

Generations:
POST /projects/{id}/generations
GET /generations/{id}
POST /generations/{id}/cancel

Prompts:
GET /projects/{id}/prompts
POST /projects/{id}/prompts
PATCH /prompts/{id}

References:
GET /projects/{id}/references
POST /projects/{id}/references
DELETE /references/{id}

Similarity:
POST /projects/{id}/similarity-checks
GET /similarity-checks/{id}

Provenance:
GET /projects/{id}/provenance
GET /projects/{id}/provenance/verify

Exports:
POST /projects/{id}/exports
GET /exports/{id}

Все API описываются в OpenAPI.

## 35. API requirements

Mutation endpoints:
- валидируют input;
- проверяют ownership;
- проверяют permissions;
- создают provenance event;
- используют стабильные response schemas;
- возвращают request/correlation ID;
- корректно обрабатывают ошибки.

Повторяемые запросы используют Idempotency-Key.

## 36. SSE

GET /api/v1/projects/{id}/events

Events:
- generation.started;
- generation.progress;
- generation.completed;
- generation.failed;
- similarity.completed;
- export.completed;
- provenance.updated.

## 37. Error model

{
  "error": {
    "code": "GENERATION_FAILED",
    "message": "Generation failed",
    "request_id": "..."
  }
}

Stack traces не выдавать клиенту.

## 38. Authentication and Authorization

Поддержать secure authentication, session/JWT strategy, password hashing при password auth, OAuth в будущем, ownership checks и permission model.

Проекты приватны по умолчанию.

Знание project_id не даёт доступа к чужому проекту.

## 39. Security

Обязательно:
- HTTPS в production;
- secure cookies при cookie auth;
- CSRF protection при cookie auth;
- CORS allowlist;
- rate limiting;
- request size limits;
- upload size limits;
- MIME validation;
- extension validation;
- image decoding validation;
- signed object URLs;
- access checks;
- malware scanning;
- SQL parameterization;
- отсутствие secrets в Git;
- provider credentials только backend;
- security headers;
- dependency scanning.

## 40. File pipeline

При upload:
1. проверить размер;
2. проверить MIME;
3. проверить формат;
4. вычислить SHA-256;
5. сохранить original;
6. создать thumbnail;
7. извлечь metadata;
8. при необходимости нормализовать EXIF;
9. security scan;
10. создать asset record;
11. создать provenance event.

Поддержать multipart/resumable upload, dedup по hash, lifecycle policies и signed URLs.

## 41. Storage

Основное production-хранилище файлов — Yandex Object Storage через S3 API. MinIO используется только для локальной разработки и тестов.

Object keys:
users/{user_id}/projects/{project_id}/assets/{asset_id}/original
users/{user_id}/projects/{project_id}/assets/{asset_id}/preview
users/{user_id}/projects/{project_id}/exports/{export_id}/...

User filename не должен быть единственным идентификатором.

## 42. Cost / Quota / Billing

Для каждой generation хранить:
- estimated_cost;
- actual_cost;
- provider;
- model;
- units при наличии;
- duration;
- status.

Поддержать daily, monthly, project и user limits.

Queue priority:
- HIGH;
- NORMAL;
- LOW.

## 43. Reliability

Generation pipeline:
- timeout;
- retry;
- idempotency;
- dead-letter strategy;
- cancellation;
- partial failure handling;
- provider outage handling;
- duplicate callback handling.

Webhook:
1. verify signature;
2. verify event ID;
3. idempotent processing;
4. persist provider event;
5. update generation;
6. emit SSE.

## 44. Backup / Disaster Recovery

Backup:
- PostgreSQL;
- object storage;
- provenance;
- configuration metadata.

Redis не является единственным источником критичных данных.

Документировать RPO, RTO и restore procedure.

## 45. Delete / Archive

Разделить archive и permanent delete.

Перед permanent delete учитывать assets, provenance, references, exports, branches и dependencies.

Удаление не должно создавать ложную историю.

## 46. Search

Поиск:
- projects;
- prompts;
- iterations;
- assets;
- materials;
- textures;
- references;
- tags.

Позже допускается pgvector.

## 47. UI screens

Минимум:
1. Login/Register;
2. Dashboard;
3. Projects;
4. Project Overview;
5. Creative Studio;
6. Prompt Studio;
7. Generation Queue;
8. Timeline;
9. References;
10. Materials;
11. Textures;
12. Style DNA;
13. Similarity Analysis;
14. Composition Analysis;
15. Provenance;
16. Creation Report;
17. Settings;
18. Usage/Quota.

## 48. Creative Studio UI

Рекомендуемый layout:
Header: Project / Mode / Generate / Export
Left: Prompt / References / Materials / Textures / Composition
Center: Canvas / Preview
Bottom: Variants / Timeline / History

Основные действия должны быть доступны без постоянного перехода между страницами.

## 49. Accessibility

Обязательно:
- keyboard navigation;
- visible focus;
- ARIA;
- semantic controls;
- sufficient contrast;
- reduced motion;
- keyboard-accessible timeline;
- accessible dialogs.

## 50. Internationalization

С первого дня:
- ru;
- en.

Backend использует UTC.
Frontend показывает пользовательскую timezone.

## 51. Testing

Backend unit tests:
- services;
- repositories;
- provider adapter;
- prompt builder;
- provenance hashing;
- similarity scoring;
- validators;
- permissions.

Integration:
- API + PostgreSQL;
- Redis queue;
- storage;
- auth;
- generation lifecycle.

Contract tests:
- YandexART через mock/fake provider;
- CI не выполняет платные реальные генерации.

Frontend:
- typecheck;
- lint;
- unit tests;
- component tests;
- build.

## 52. CI/CD

Каждый Pull Request должен выполнять:
- gofmt;
- go vet ./...;
- go test ./...;
- go test -race ./...;
- go build ./...;
- backend lint;
- frontend typecheck;
- frontend lint;
- frontend test;
- frontend build;
- Docker build;
- migration validation.

После каждого изменения кода обязательно проверять GitHub Actions.

Definition of Done невозможен при красном CI.

## 53. GitHub development rule

AI-агент и разработчик обязаны:
1. изучить изменение;
2. выполнить локальные проверки;
3. сделать commit/PR;
4. дождаться GitHub Actions;
5. проверить required checks;
6. при failure найти причину;
7. исправить;
8. повторить CI;
9. только после green CI считать изменение завершённым.

Нельзя сообщать «готово», если CI не проверен.

## 54. Definition of Done

Задача выполнена только если:
- backend реализован;
- frontend реализован;
- migrations готовы;
- tests готовы;
- API contract обновлён;
- OpenAPI обновлён;
- error/loading states реализованы;
- retry/cancel предусмотрены;
- permissions проверены;
- provenance event добавлен;
- документация обновлена;
- secrets отсутствуют;
- нет блокирующих TODO;
- нет accidental files;
- build проходит;
- tests проходят;
- GitHub CI зелёный.

## 55. AI Agent Development Rules

AI coding agent должен:
1. сначала изучить существующий код;
2. не ломать API без миграционного плана;
3. не создавать дублирующую архитектуру;
4. не использовать прямой SQL из AI tool;
5. соблюдать provider abstraction;
6. писать тесты вместе с функциональностью;
7. сохранять backwards compatibility;
8. не коммитить секреты;
9. не удалять историю без явного требования;
10. не подменять human actions AI actions;
11. добавлять provenance для творческих mutation operations;
12. проверять CI после каждого изменения;
13. исправлять красный CI до завершения;
14. не оставлять блокирующие TODO;
15. документировать архитектурно значимые решения.

## 56. MVP phases

### Phase 1 — Foundation
Go, Vue 3, PostgreSQL, Redis, storage, auth, projects, base UI, CI/CD.

### Phase 2 — YandexART
provider adapter, generation, worker, queue, seed, parameters, images, timeline basics.

### Phase 3 — Creative Process
prompts, prompt versions, iterations, branches, references, human actions.

### Phase 4 — Provenance
event log, hash chain, verification, manifest, Creation Report.

### Phase 5 — Intelligence
similarity, composition analysis, composition mutation, reference influence, AI critic, fact checker.

### Phase 6 — Creative Library
Style DNA, Materials, Textures, Asset DNA, Personal Visual Language.

### Phase 7 — Advanced Export
masks, layered workflow, PSD, professional export, extended evidence package.

## 57. Что не делать на старте

Не реализовывать в MVP:
- собственную image-generation model;
- собственный большой ML stack;
- Kubernetes;
- микросервисы;
- десятки providers;
- сложный PSD engine;
- собственную vector database;
- social network;
- marketplace;
- публичный feed.

Сначала доказать ценность creative provenance + iterative AI studio.

## 58. Observability

Использовать structured logging, request ID, trace ID, metrics, OpenTelemetry, Prometheus и Grafana.

Минимальные metrics:
- generation_total;
- generation_success_total;
- generation_failed_total;
- generation_duration_seconds;
- provider_error_total;
- queue_depth;
- queue_latency;
- similarity_duration;
- export_duration;
- API_latency;
- API_error_total.

## 59. Performance targets

Цели MVP:
- API p95 простых read requests < 300 ms без external provider;
- API p95 mutation requests < 500 ms без ожидания async generation;
- generation request всегда асинхронный;
- thumbnails отделены от originals;
- frontend не блокируется ожиданием generation.

Точные production SLO уточняются после измерений.

## 60. Data integrity

Критичные mutation operations транзакционны.

Желательный порядок:
DB transaction:
- domain mutation;
- human action;
- provenance event;
- commit.

Hash provenance вычисляется из канонизированного payload.

## 61. Versioning

Версионировать:
- API;
- prompt;
- style profile;
- asset;
- composition representation;
- provenance schema;
- similarity algorithm;
- provider adapter metadata.

Старые similarity results не перезаписывать при появлении нового алгоритма.

## 62. Reproducibility

Для каждой generation сохранять:
- provider;
- model;
- model version;
- seed;
- prompt;
- negative prompt;
- aspect ratio;
- parameters;
- references;
- material choices;
- texture choices;
- timestamp.

Повторяемость результата не гарантируется без соответствующей гарантии provider.

## 63. Privacy

По умолчанию private:
- projects;
- prompts;
- references;
- generated assets;
- provenance;
- exports.

Публичный доступ только по явному действию пользователя.

## 64. Ownership checks

Каждый endpoint с project_id, iteration_id, asset_id, generation_id, reference_id или export_id обязан проверять принадлежность ресурса текущему пользователю.

Frontend authorization недостаточно.

## 65. Database migrations

Все schema changes выполняются через versioned migrations.

Запрещено вручную менять production schema без migration.

Migration должна быть протестирована и документирована при breaking change.

## 66. OpenAPI

OpenAPI — основной контракт backend/frontend.

Из OpenAPI генерировать TypeScript types/client.

Изменение API требует обновления:
- OpenAPI;
- backend;
- frontend;
- tests;
- documentation.

## 67. Local development

Docker Compose поднимает:
- PostgreSQL;
- Redis;
- MinIO;
- backend;
- worker;
- frontend.

Локальная разработка не должна требовать production credentials.

При отсутствии YandexART credentials использовать mock mode.

## 68. Environment configuration

Пример переменных:
APP_ENV
APP_PORT
DATABASE_URL
REDIS_URL
S3_ENDPOINT
S3_REGION
S3_BUCKET
S3_ACCESS_KEY
S3_SECRET_KEY
# Production: Yandex Object Storage S3 endpoint/bucket
YANDEXART_API_KEY
YANDEXART_FOLDER_ID
JWT_SECRET

Secrets передаются через environment или secret manager.

## 69. Acceptance criteria MVP

Пользователь может:
1. зарегистрироваться;
2. создать проект;
3. написать идею;
4. создать prompt;
5. получить generation;
6. увидеть результат;
7. сохранить варианты;
8. выбрать вариант;
9. создать новую итерацию;
10. увидеть timeline;
11. изменить prompt;
12. повторить generation;
13. добавить reference;
14. выбрать material/texture;
15. увидеть human contribution;
16. получить provenance;
17. проверить provenance;
18. экспортировать Creation Report;
19. архивировать проект;
20. открыть проект позже без потери истории.

## 70. Acceptance criteria provenance

Тестовый проект должен поддерживать минимум 10 событий.

Проверить:
- каждый event имеет hash;
- кроме первого, каждый имеет parent_hash;
- изменение payload делает verification invalid;
- изменение старого event обнаруживается;
- удаление event обнаруживается;
- новый branch сохраняет связь с parent iteration.

## 71. Acceptance criteria AI

AI assistant:
- не имеет SQL access;
- использует только разрешённые tools;
- различает human/AI input;
- объясняет recommendation;
- показывает confidence;
- поддерживает Apply/Edit/Ignore;
- записывает AI action;
- не меняет immutable history;
- не превращает UNKNOWN в факт.

## 72. Acceptance criteria similarity

Similarity engine:
- выдаёт visual/composition/semantic/style scores отдельно;
- хранит algorithm version;
- хранит timestamp;
- хранит search scope;
- не выдаёт юридическое заключение;
- поддерживает повторную проверку новым алгоритмом без удаления старого результата.

## 73. Acceptance criteria security

Проверить:
- unauthorized project access;
- asset access;
- invalid upload;
- oversized upload;
- malformed image;
- rate limit;
- CSRF при cookie auth;
- secret exposure;
- SQL injection;
- path traversal;
- signed URL expiry.

## 74. Future extensions

После MVP допускаются:
- дополнительные image providers;
- custom vision models;
- embeddings;
- pgvector;
- external similarity sources;
- advanced segmentation;
- browser canvas editing;
- PSD export;
- collaborative projects;
- team workspaces;
- organization permissions;
- billing;
- marketplace после отдельной product/security/legal спецификации.

## 75. Product positioning

Рабочее позиционирование:
AI Creative Studio + Proof of Creation

Основная идея:
Не просто сгенерировать изображение, а сохранить и доказуемо описать путь от идеи до результата.

Ценности:
Create. Refine. Prove.

## 76. Критические архитектурные правила

1. YandexART — provider, а не доменная модель.
2. Immutable creative history.
3. Branching вместо перезаписи.
4. Human input отделён от AI suggestions.
5. Каждая creative mutation operation имеет provenance.
6. Provenance не равен audit log.
7. Similarity не равна юридической экспертизе.
8. Unknown rights остаются unknown.
9. AI recommendations user-overridable.
10. AI tools вместо прямого SQL.
11. Private by default.
12. Provider credentials только backend.
13. Async generation.
14. Idempotency и retry.
15. OpenAPI как API contract.
16. PostgreSQL как source of truth.
17. Redis не является source of truth.
18. CI green обязателен.
19. Не обещать абсолютную уникальность.
20. Не подделывать человеческий вклад.

## 77. Приоритет реализации

P0:
- foundation;
- auth;
- projects;
- database;
- storage;
- CI;
- YandexART adapter;
- generation;
- timeline;
- provenance.

P1:
- prompts;
- references;
- human contribution;
- materials;
- textures;
- Creation Report.

P2:
- similarity;
- composition engine;
- AI critic;
- fact checker.

P3:
- Style DNA;
- Asset DNA;
- Personal Visual Language;
- advanced export.

## 78. Первый технический milestone

Первый milestone должен создать вертикальный срез:

Vue UI
→ Go REST API
→ PostgreSQL
→ Redis queue
→ Go worker
→ YandexART adapter/mock
→ Object Storage
→ Generation result
→ Timeline
→ Provenance event
→ Verification

После этого расширять функциональность по фазам.

## 79. Финальное требование

Платформа должна сохранять не только результат генерации, но и историю творческих решений, человеческий вклад, AI-влияние, исходные данные, версии, проверки и доказательства процесса.

Это ключевое архитектурное требование проекта и не должно быть потеряно при последующих рефакторингах.

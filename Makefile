# recruitmate 根目录统一入口
# 前端：pnpm dev:web / dev:careers
# 后端与基础设施：make infra-up && make api-migrate && make api-seed && make api-dev

.PHONY: infra-up infra-down api-dev api-migrate api-seed api-test api-build frontend-install build test

# 优先使用 Docker Compose；无 Docker 的机器（如 macOS 无 Docker Desktop）回退到 brew services
infra-up:
	@if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then \
		docker compose up -d postgres redis minio; \
	else \
		brew services start postgresql@17 redis minio; \
	fi

infra-down:
	@if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then \
		docker compose down; \
	else \
		brew services stop postgresql@17 redis minio; \
	fi

api-dev:
	$(MAKE) -C apps/api dev

api-migrate:
	$(MAKE) -C apps/api migrate-up

api-seed:
	$(MAKE) -C apps/api seed

api-test:
	$(MAKE) -C apps/api test

api-build:
	$(MAKE) -C apps/api build

frontend-install:
	pnpm install

build: api-build
	pnpm build

test: api-test
	pnpm -r test

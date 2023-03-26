migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)
migrate-up:
	migrate -database "mysql://user:password@tcp(localhost:3306)/dev" -path migrations up
migrate-down:
	migrate -database "mysql://user:password@tcp(localhost:3306)/dev" -path migrations down
dev:
	air -build.include_ext mustache,go,html,js,ts
minio-dev:
	MINIO_USER=minio MINIO_PASSWORD=minio123 minio server ./minio

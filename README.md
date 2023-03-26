# Pigeon

MPA Messaging App written in Go. Just for fun. GFMM Stack 😉  
[Go Fiber](https://gofiber.io) backend serving up [Mustache](https://mustache.github.io) HTML templates, MySQL Database.

## Getting Started

### Requirements
- [Go 1.19](https://go.dev) - Unsure if newer versions will work, using V2 Go Modules
- [Minio](https://github.com/minio/minio) - local S3
- [Air](https://github.com/cosmtrek/air) - hot reload
- [Docker, docker compose](https://www.docker.com/) - MySQL
- [golang-migrate](https://github.com/golang-migrate/migrate) - DB Migrations

### Running locally

#### Environment

Copy example env file
```bash
cp .env.example .env
```

#### Github OAuth App

See the following instructions for getting a Github oAuth app setup.  
You can use `http://localhost:4872` for your "HomePage URL"
and `http://localhost:4872/auth/github/callback` for your "Authorization callback URL"  

[Github - Creating an OAuth App](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app)

#### JWT

Create JWT RS-256 public and private keys, add these to your `.env`
```bash
ssh-keygen -t rsa -b 4096 -m PEM -f jwtRS256.key
openssl rsa -in jwtRS256.key -pubout -outform PEM -out jwtRS256.key.pub
# This will output your JWT_PUBLIC_KEY
cat jwtRS256.key.pub
# This will output your JWT_SECRET_KEY
cat jwtRS256.key
```
*Make sure when adding JWT keys to surround them in `''` so the multi lines are captured*

#### Dependencies

Install dependencies
```bash
go get
```

#### Starting the App

Start MySQL with Docker
```bash
docker compose up
```

Run migrations
```
make migrate-up
```

Start Minio (local S3)
```
make minio-dev
```

Start Go server with hot reload
```
make dev
```

### Database

Migrations are done using [golang-migrate](https://github.com/golang-migrate/migrate). They have good documentation

For helper commands, I have added
- `make migrate-down` Apply all down migrations
- `make migrate-up` Apply all migrations
- `make migrate-create NAME=<migration_name>` - Create a new migration


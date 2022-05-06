
docker-compose build

docker network create popug-jira

docker-compose run oauth rake db:create
docker-compose run oauth rake db:migrate

docker-compose run tasktracker migrate -path /app/db/migration -database "postgres://postgres:password@db:5432/postgres?sslmode=disable" -verbose up

docker-compose up




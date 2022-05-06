
docker-compose build

docker network create popug-jira

docker-compose run oauth rake db:create
docker-compose run oauth rake db:migrate

docker-compose up




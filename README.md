[![Go CI](https://github.com/mitanuriel/GoSearch/actions/workflows/actions.yml/badge.svg?branch)](https://github.com/mitanuriel/GoSearch/actions/workflows/actions.yml)


## Run program when in backend directory:
    go run .

## With docker
## First build:
    docker compose -f docker-compose.dev.yml build
## The run up:
    docker compose -f docker-compose.dev.yml up

## If you want to rebuild with no cache:
    docker compose -f docker-compose.dev.yml build --no-cache

## To take it down:
    docker compose -f docker-compose.dev.yml down
# Gulp
Gulp is a command-line RSS feed aggregator written in Go. It allows you to register users and RSS feeds, as well as follow feeds added by other users. Posts from aggregated feeds are collected into a PostgreSQL database so that you can browse them right in your terminal.

## Features
- Continually running aggregator that scrapes feeds and saves posts
- Multi-user support
- Add RSS feeds
- Follow and unfollow feeds added by other users
- Browse posts from followed feeds

## Prerequisites
- Go (1.22 or later)
- PostgreSQL (15 or later)

## Setup
1. Install Go and PostgreSQL
2. Start Postgres server (replace 16 with your version)
MacOS (Homebrew): 
```
brew services start postgresql@16
```
Linux/WSL: 
```
sudo service postgresql start
```
Linux with systemd: 
```
sudo systemctl start postgresql
```
3. Connect to Postgres server
    - MacOS: 
        ```
        psql postgres
        ```
    - Linux/WSL: 
        ```
        sudo -u postgres psql
        ```
        Then set a password for the `postgres` user:
        ``` 
        ALTER USER postgres PASSWORD 'postgres';
        ```

4. Create database, then exit with `\q`:
    ```CREATE DATABASE gulp;```
5. Install Gulp
    ```go install github.com/<your-username>/gulp@latest```
6. Run the database migrations:
    ```
    go install github.com/pressly/goose/v3/cmd/goose@latest
    git clone https://github.com/<your-username>/gulp.git
    cd gulp/sql/schema
    goose postgres "postgres://postgres:postgres@localhost:5432/gulp" up
    ```

    On MacOS, use `postgres://<you-mac-username>:@localhost:5432/gulp`
7. Create config file at ~/.gulpconfig.json
    ```
    {
        "db_url": "postgres://postgres:postgres@localhost:5432/gulp?sslmode=disable"
    }
    ```
    On MacOS, use `postgres://<you-mac-username>:@localhost:5432/gulp?sslmode=disable`


## License
MIT
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
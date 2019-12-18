This directory contains everything related to our Docker setup (except `docker-compose.yml`,
as that's placed in the root directory canonically).

Files/directories:

- `.alice`: Data directory for the `alice` node spun up by `docker-compose`
- `.bob`: Data directory for the `bob` node spun up by `docker-compose`
- `lnd`: Dockerfile and start script for our LND image

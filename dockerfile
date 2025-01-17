# Use the official Go 1.23 image
FROM golang:1.23

# Install migrate CLI and debugging tools
RUN apt-get update && apt-get install -y wget postgresql-client make tree findutils \
    && wget https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz \
    && tar -xzf migrate.linux-amd64.tar.gz \
    && mv migrate /usr/local/bin/ \
    && rm migrate.linux-amd64.tar.gz \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Install PostgreSQL driver
RUN go get github.com/lib/pq

# Build the application
RUN go build -o api ./cmd/api

# Create detailed entrypoint script
RUN echo '#!/bin/bash\n\
set -e\n\
\n\
# Extensive Debugging\n\
echo "Current working directory:"\n\
pwd\n\
\n\
echo "Searching for migration files in current directory:"\n\
find . -name "*.up.sql" -o -name "*.down.sql"\n\
\n\
# Check if migrations exist in multiple possible locations\n\
MIGRATION_DIRS=(\n\
    "internal/migrations"\n\
    "./internal/migrations"\n\
    "migrations"\n\
    "./migrations"\n\
)\n\
\n\
MIGRATION_PATH=""\n\
for dir in "${MIGRATION_DIRS[@]}"; do\n\
    if [ -d "$dir" ] && [ "$(ls -A "$dir"/*.up.sql 2>/dev/null)" ]; then\n\
        MIGRATION_PATH="$dir"\n\
        break\n\
    fi\n\
done\n\
\n\
if [ -z "$MIGRATION_PATH" ]; then\n\
    echo "No migration files found in expected directories!"\n\
    exit 1\n\
fi\n\
\n\
echo "Using migration path: $MIGRATION_PATH"\n\
echo "Migration files found:"\n\
ls "$MIGRATION_PATH"/*.up.sql\n\
\n\
# Wait for PostgreSQL to be ready\n\
until pg_isready -h postgres -p 5432; do\n\
  echo "Waiting for PostgreSQL to be ready..."\n\
  sleep 2\n\
done\n\
\n\
# Run database migrations with verbose output\n\
echo "Attempting to run migrations..."\n\
migrate -verbose -path="$MIGRATION_PATH" -database "$DATABASE_URL" up\n\
\n\
# Capture migration result\n\
MIGRATE_EXIT_CODE=$?\n\
\n\
if [ $MIGRATE_EXIT_CODE -ne 0 ]; then\n\
    echo "Migration failed with exit code $MIGRATE_EXIT_CODE"\n\
    exit $MIGRATE_EXIT_CODE\n\
fi\n\
\n\
# Start the application\n\
echo "Starting the application..."\n\
exec ./api\n' > /entrypoint.sh \
    && chmod +x /entrypoint.sh

# Expose the app port
EXPOSE 8080

# Use the entrypoint script
ENTRYPOINT ["/entrypoint.sh"]
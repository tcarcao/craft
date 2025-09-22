FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o /server ./cmd/server

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    graphviz \
    openjdk-11-jre \
    wget \
    fontconfig \
    fonts-dejavu \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

# Create directories
WORKDIR /opt/plantuml

# Install PlantUML
RUN wget "https://github.com/plantuml/plantuml/releases/download/v1.2024.0/plantuml-1.2024.0.jar" -O plantuml.jar

# Install AWS PlantUML library
RUN git clone https://github.com/awslabs/aws-icons-for-plantuml.git && \
    cd aws-icons-for-plantuml && \
    mkdir -p /opt/plantuml/awslib && \
    cp -r dist/* /opt/plantuml/awslib/

# Copy styles file
COPY assets/styles/styles.puml /opt/plantuml/styles/styles.puml

# Create the plantuml script with include path
RUN echo '#!/bin/sh' > /usr/local/bin/plantuml && \
    echo 'java -Djava.awt.headless=true -jar /opt/plantuml/plantuml.jar -charset UTF-8 -DPLANTUML_LIMIT_SIZE=8192 -includespath /opt/plantuml "$@"' >> /usr/local/bin/plantuml && \
    chmod +x /usr/local/bin/plantuml

# Verify the installation works
RUN plantuml -version && \
    echo "@startuml\nA->B: test\n@enduml" | plantuml -pipe -tpng > /dev/null

# Copy the compiled application
COPY --from=builder /server /usr/local/bin/

# Set the working directory for the application
WORKDIR /app

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
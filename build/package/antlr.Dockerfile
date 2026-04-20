FROM eclipse-temurin:11-jre-alpine

ARG ANTLR_VERSION=4.13.2

RUN apk add --no-cache curl

RUN curl -sLo /usr/local/lib/antlr.jar \
    "https://www.antlr.org/download/antlr-${ANTLR_VERSION}-complete.jar"

ENTRYPOINT ["java", "-jar", "/usr/local/lib/antlr.jar"]

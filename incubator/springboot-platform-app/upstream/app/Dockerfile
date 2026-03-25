# Dockerfile for inventory-api Spring Boot application
#
# Build: docker build -t inventory-api:local .
# Run:   docker run -p 8080:8080 inventory-api:local

FROM eclipse-temurin:21-jre-alpine

WORKDIR /app

# Copy the built JAR file
COPY target/inventory-api-*.jar app.jar

# Default port (can be overridden via SPRING_PROFILES_ACTIVE and server.port)
EXPOSE 8080 8081

# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]

# Dockerfile for order-api Spring Boot application
#
# Build: docker build -t order-api:local .
# Run:   docker run -p 8080:8080 order-api:local

FROM eclipse-temurin:21-jre-alpine

WORKDIR /app

# Copy the built JAR file
COPY target/order-api-*.jar app.jar

# Default port (can be overridden via SPRING_PROFILES_ACTIVE and server.port)
EXPOSE 8080 8081

# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]

FROM openpolicyagent/opa:latest

WORKDIR /app

# Copy policy files
COPY policy/ /app/policy/

# Expose OPA port
EXPOSE 8181

# Run OPA server
CMD ["run", "--server", "--addr", ":8181", "/app/policy"]


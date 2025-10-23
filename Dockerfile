# ABOUTME: Docker image for running ACP agents in isolated containers
# ABOUTME: Installs Node.js, git, curl, and @zed-industries/claude-code-acp agent
FROM node:20-slim

# Install dependencies the agent might need
RUN apt-get update && apt-get install -y \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /workspace

# Install the ACP agent globally
RUN npm install -g @zed-industries/claude-code-acp

# The agent expects stdio communication
ENTRYPOINT ["npx", "@zed-industries/claude-code-acp"]

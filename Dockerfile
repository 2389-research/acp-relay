# ABOUTME: Generic runtime image for ACP agents in isolated containers
# ABOUTME: Provides Node.js, Python, git, and common tools - agent command configured at runtime
FROM node:20-slim

# Install runtimes and tools that agents commonly need
RUN apt-get update && apt-get install -y \
    git \
    curl \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /workspace

# No ENTRYPOINT or CMD - command comes from container config at runtime
# This allows one image to run multiple agent types:
#   - npx @zed-industries/claude-code-acp
#   - python -m my_custom_agent
#   - /usr/local/bin/codex-agent
# The relay's container.Config.Cmd will specify which agent to run

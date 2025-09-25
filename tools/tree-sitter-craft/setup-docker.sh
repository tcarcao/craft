#!/bin/bash

# Docker/Podman setup script for tree-sitter WASM builds
# This script detects available container runtime and sets up the docker executable if needed

setup_docker() {
    echo "🐳 Setting up container runtime for WASM builds..."
    
    # Check if docker is already available
    DOCKER_PATH=$(which docker 2>/dev/null)
    if [ -n "$DOCKER_PATH" ]; then
        echo "✅ Docker found at: $DOCKER_PATH"
        echo "   Version: $(docker --version)"
        return 0
    fi
    
    # Check if podman is available using which
    PODMAN_PATH=$(which podman 2>/dev/null)
    if [ -n "$PODMAN_PATH" ]; then
        echo "✅ Podman found at: $PODMAN_PATH"
        echo "   Version: $(podman --version)"
        
        # Create docker executable if it doesn't exist
        if [ ! -f "./docker" ]; then
            echo "📝 Creating docker wrapper for podman..."
            cat > ./docker << EOF
#!/bin/bash
# Wrapper script to use podman instead of docker
exec $PODMAN_PATH "\$@"
EOF
            chmod +x ./docker
            echo "✅ Created ./docker wrapper pointing to $PODMAN_PATH"
        else
            echo "✅ Docker wrapper already exists"
        fi
        return 0
    fi
    
    # Check common installation paths for podman if which didn't find it
    COMMON_PODMAN_PATHS=(
        "/opt/podman/bin/podman"
        "/usr/local/bin/podman"
        "/usr/bin/podman"
        "$HOME/.local/bin/podman"
    )
    
    for path in "${COMMON_PODMAN_PATHS[@]}"; do
        if [ -f "$path" ]; then
            echo "✅ Podman found at: $path"
            echo "   Version: $($path --version)"
            
            # Create docker executable pointing to this podman
            if [ ! -f "./docker" ]; then
                echo "📝 Creating docker wrapper for podman..."
                cat > ./docker << EOF
#!/bin/bash
# Wrapper script to use podman instead of docker
exec $path "\$@"
EOF
                chmod +x ./docker
                echo "✅ Created ./docker wrapper pointing to $path"
            else
                echo "✅ Docker wrapper already exists"
            fi
            return 0
        fi
    done
    
    # No container runtime found
    echo "❌ No container runtime found!"
    echo ""
    echo "Searched for docker and podman in:"
    echo "  - PATH directories (using 'which')"
    echo "  - /opt/podman/bin/podman"
    echo "  - /usr/local/bin/podman"
    echo "  - /usr/bin/podman"
    echo "  - $HOME/.local/bin/podman"
    echo ""
    echo "To build WASM, you need either Docker or Podman installed:"
    echo ""
    echo "Option 1 - Install Docker:"
    echo "  - Download from: https://www.docker.com/products/docker-desktop"
    echo ""
    echo "Option 2 - Install Podman:"
    echo "  - macOS: brew install podman"
    echo "  - Linux: See https://podman.io/getting-started/installation"
    echo ""
    echo "After installation, run this script again."
    return 1
}

# Export function so it can be called from other scripts
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    # Script is being run directly
    setup_docker
else
    # Script is being sourced
    export -f setup_docker
fi
#!/bin/bash

# Script to manage the shared API network for inter-container communication

NETWORK_NAME="shared-api-network"

case "$1" in
    "create")
        echo "Creating shared network: $NETWORK_NAME"
        docker network create $NETWORK_NAME
        echo "Network created successfully!"
        echo ""
        echo "This network allows containers from different projects to communicate."
        echo "Make sure other API projects also use this network in their docker-compose files."
        ;;
    "remove")
        echo "Removing shared network: $NETWORK_NAME"
        docker network rm $NETWORK_NAME
        echo "Network removed successfully!"
        ;;
    "inspect")
        echo "Inspecting shared network: $NETWORK_NAME"
        docker network inspect $NETWORK_NAME
        ;;
    "list")
        echo "Connected containers in network: $NETWORK_NAME"
        docker network inspect $NETWORK_NAME --format='{{range .Containers}}{{.Name}} ({{.IPv4Address}}){{println}}{{end}}'
        ;;
    *)
        echo "Usage: $0 {create|remove|inspect|list}"
        echo ""
        echo "Commands:"
        echo "  create  - Create the shared network"
        echo "  remove  - Remove the shared network"
        echo "  inspect - Show detailed network information"
        echo "  list    - List connected containers and their IPs"
        echo ""
        echo "Example usage:"
        echo "  $0 create   # Run this once to set up the network"
        echo "  $0 list     # Check which containers are connected"
        exit 1
        ;;
esac 
#!/bin/bash
#version 1.0
show_usage(){
    echo "Usage: $0 [options]"
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --verbose  Enable verbose output"
}

cmd="$1"

case "$cmd" in
    -h|--help)
        show_usage
        exit 0
        ;;
    -v|--verbose)
        verbose=true
        ;;
    *)
        echo "Invalid option: $cmd"
        show_usage
        exit 1
        ;;
esac
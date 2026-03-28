#!/bin/bash
# Script to download CodeMirror assets locally for production use

set -e

# Create directories
mkdir -p web/static/css/codemirror
mkdir -p web/static/js/codemirror

# Base URL for CodeMirror 5.65.17
BASE_URL="https://cdnjs.cloudflare.com/ajax/libs/codemirror/5.65.17"

echo "Downloading CodeMirror assets..."

# CSS files
echo "Downloading CSS files..."
curl -s -o web/static/css/codemirror/codemirror.min.css "${BASE_URL}/codemirror.min.css"
curl -s -o web/static/css/codemirror/dracula.min.css "${BASE_URL}/theme/dracula.min.css"

# JS files
echo "Downloading JS files..."
curl -s -o web/static/js/codemirror/codemirror.min.js "${BASE_URL}/codemirror.min.js"
curl -s -o web/static/js/codemirror/xml.min.js "${BASE_URL}/mode/xml/xml.min.js"
curl -s -o web/static/js/codemirror/javascript.min.js "${BASE_URL}/mode/javascript/javascript.min.js"
curl -s -o web/static/js/codemirror/css.min.js "${BASE_URL}/mode/css/css.min.js"
curl -s -o web/static/js/codemirror/htmlmixed.min.js "${BASE_URL}/mode/htmlmixed/htmlmixed.min.js"

# Addon files for hinting and autocomplete
echo "Downloading addon files..."
curl -s -o web/static/js/codemirror/show-hint.js "${BASE_URL}/addon/hint/show-hint.js"
curl -s -o web/static/css/codemirror/show-hint.css "${BASE_URL}/addon/hint/show-hint.css"

# Optional addons for enhanced editing
curl -s -o web/static/js/codemirror/matchbrackets.js "${BASE_URL}/addon/edit/matchbrackets.js"
curl -s -o web/static/js/codemirror/closebrackets.js "${BASE_URL}/addon/edit/closebrackets.js"
curl -s -o web/static/js/codemirror/closetag.js "${BASE_URL}/addon/edit/closetag.js"

echo "Done! Files downloaded to:"
echo "  - web/static/css/codemirror/"
echo "  - web/static/js/codemirror/"

echo ""
echo "To use local files instead of CDN, update the email_templates.tmpl file:"
echo "1. Change CSS links from CDN to:"
echo "   <link rel=\"stylesheet\" href=\"/gui/static/css/codemirror/codemirror.min.css\">"
echo "   <link rel=\"stylesheet\" href=\"/gui/static/css/codemirror/dracula.min.css\">"
echo "   <link rel=\"stylesheet\" href=\"/gui/static/css/codemirror/show-hint.css\">"
echo ""
echo "2. Change JS script tags from CDN to:"
echo "   <script src=\"/gui/static/js/codemirror/codemirror.min.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/xml.min.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/javascript.min.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/css.min.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/htmlmixed.min.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/show-hint.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/matchbrackets.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/closebrackets.js\"></script>"
echo "   <script src=\"/gui/static/js/codemirror/closetag.js\"></script>"
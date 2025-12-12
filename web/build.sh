#!/bin/bash
set -e

# Clean and create dist directory
rm -rf dist
mkdir -p dist/components/wgsl-minifier

# Copy files and ensure they're readable
cp src/styles.css src/main.js dist/
cp src/components/wgsl-minifier/* dist/components/wgsl-minifier/
chmod -R a+r dist/
cp -r public/* dist/
cp node_modules/boredom/dist/boreDOM.full.js dist/boreDOM.js

# Create index.html with import map
cat > dist/index.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
  <script type="importmap">{ "imports": { "boredom": "./boreDOM.js" } }</script>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>WGSL Minifier</title>
  <link rel="stylesheet" href="./styles.css">
  <script src="./main.js" type="module"></script>
</head>
<body>
  <header>
    <h1>WGSL Minifier</h1>
    <p>Compress your WebGPU shaders</p>
  </header>
  <main>
    <wgsl-minifier></wgsl-minifier>
  </main>
  <footer>
    <a href="https://codeberg.org/saruga/wgsl-minifier" target="_blank" rel="noopener">Source</a>
    <span class="separator"></span>
    <p>Made with&nbsp;<a href="https://hugodaniel.com/pages/boreDOM/">boreDOM</a> <span class="sleepy">🥱</span></p>
  </footer>
</body>
</html>
HTMLEOF

echo "Build complete: dist/"

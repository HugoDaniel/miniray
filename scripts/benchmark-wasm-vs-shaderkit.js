#!/usr/bin/env node
/**
 * Benchmark comparing miniray WASM vs shaderkit (both pure JS)
 * This gives a fairer speed comparison without subprocess overhead
 */

const fs = require('fs');
const path = require('path');

// Set up globals required by wasm_exec.js
globalThis.require = require;
globalThis.fs = require('fs');
globalThis.path = require('path');
globalThis.TextEncoder = require('util').TextEncoder;
globalThis.TextDecoder = require('util').TextDecoder;
globalThis.performance ??= require('perf_hooks').performance;
globalThis.crypto ??= require('crypto');

// Load wasm_exec.js
require('../npm/miniray/wasm_exec.js');

const { minify: shaderkitMinify } = require('shaderkit');

// Configuration
const ITERATIONS = 100;
const TESTDATA_DIR = process.env.TESTDATA_DIR || 'testdata';

// Colors
const GREEN = '\x1b[32m';
const BLUE = '\x1b[34m';
const NC = '\x1b[0m';

function findWgslFiles(dir) {
    const files = [];
    if (!fs.existsSync(dir)) return files;
    for (const entry of fs.readdirSync(dir)) {
        const fullPath = path.join(dir, entry);
        if (fs.statSync(fullPath).isFile() && entry.endsWith('.wgsl')) {
            files.push(fullPath);
        }
    }
    return files.sort();
}

function measureTime(fn, iterations) {
    // Warm up
    for (let i = 0; i < 3; i++) fn();

    const start = performance.now();
    for (let i = 0; i < iterations; i++) {
        fn();
    }
    const end = performance.now();
    return (end - start) / iterations;
}

async function loadMinirayWasm() {
    const go = new Go();
    const wasmPath = path.join(__dirname, '..', 'npm', 'miniray', 'miniray.wasm');
    const wasmBuffer = fs.readFileSync(wasmPath);
    const result = await WebAssembly.instantiate(wasmBuffer, go.importObject);
    go.run(result.instance);

    // Wait for initialization
    for (let i = 0; i < 50; i++) {
        if (globalThis.__miniray) return globalThis.__miniray;
        await new Promise(r => setTimeout(r, 10));
    }
    throw new Error('WASM init failed');
}

async function main() {
    console.log(`\n${BLUE}=== WASM Speed Benchmark: miniray vs shaderkit ===${NC}`);
    console.log(`iterations per file: ${ITERATIONS}\n`);

    // Load miniray WASM
    console.log('Loading miniray WASM...');
    const miniray = await loadMinirayWasm();
    console.log(`miniray version: ${miniray.version}`);

    const shaderkitPkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'node_modules', 'shaderkit', 'package.json'), 'utf8'));
    console.log(`shaderkit version: ${shaderkitPkg.version}\n`);

    // Find test files
    let files = findWgslFiles(TESTDATA_DIR);
    files = [...files, ...findWgslFiles(path.join(TESTDATA_DIR, 'compute.toys'))];

    console.log('| File                         | Original | miniray | shaderkit | miniray | shaderkit |  miniray | shaderkit |');
    console.log('|                              |    bytes |   bytes |     bytes |       % |         % |     time |      time |');
    console.log('|------------------------------|----------|---------|-----------|---------|-----------|----------|-----------|');

    let totalOrig = 0, totalMiniray = 0, totalShaderkit = 0;
    let totalMinirayTime = 0, totalShaderkitTime = 0;

    for (const file of files) {
        const filename = path.basename(file);
        const code = fs.readFileSync(file, 'utf8');
        const origSize = Buffer.byteLength(code, 'utf8');

        // Benchmark miniray WASM (with tree-shaking enabled by default)
        let miniraySize, minirayTime;
        try {
            const minirayResult = miniray.minify(code);  // defaults: all optimizations + tree-shaking
            miniraySize = Buffer.byteLength(minirayResult.code, 'utf8');
            minirayTime = measureTime(() => {
                miniray.minify(code);
            }, ITERATIONS);
        } catch (e) {
            miniraySize = null;
            minirayTime = null;
        }

        // Benchmark shaderkit
        let shaderkitSize, shaderkitTime;
        try {
            const shaderkitResult = shaderkitMinify(code, { mangle: true });
            shaderkitSize = Buffer.byteLength(shaderkitResult, 'utf8');
            shaderkitTime = measureTime(() => {
                shaderkitMinify(code, { mangle: true });
            }, ITERATIONS);
        } catch (e) {
            shaderkitSize = null;
            shaderkitTime = null;
        }

        totalOrig += origSize;
        if (miniraySize) { totalMiniray += miniraySize; totalMinirayTime += minirayTime; }
        if (shaderkitSize) { totalShaderkit += shaderkitSize; totalShaderkitTime += shaderkitTime; }

        const fmtSize = (s) => s === null ? 'ERR' : s.toString();
        const fmtPct = (o, m) => m === null ? '-' : Math.round(100 - (m * 100 / o)) + '%';
        const fmtTime = (t) => t === null ? '-' : t.toFixed(3) + 'ms';

        console.log(`| ${filename.padEnd(28)} | ${origSize.toString().padStart(8)} | ${fmtSize(miniraySize).padStart(7)} | ${fmtSize(shaderkitSize).padStart(9)} | ${fmtPct(origSize, miniraySize).padStart(7)} | ${fmtPct(origSize, shaderkitSize).padStart(9)} | ${fmtTime(minirayTime).padStart(8)} | ${fmtTime(shaderkitTime).padStart(9)} |`);
    }

    console.log(`\n${BLUE}=== Summary ===${NC}\n`);
    console.log(`${GREEN}miniray (WASM):${NC}`);
    console.log(`  Total: ${totalOrig} -> ${totalMiniray} bytes (${(100 - totalMiniray * 100 / totalOrig).toFixed(1)}% reduction)`);
    console.log(`  Avg time per file: ${(totalMinirayTime / files.length).toFixed(3)}ms`);

    console.log(`${GREEN}shaderkit (JS):${NC}`);
    console.log(`  Total: ${totalOrig} -> ${totalShaderkit} bytes (${(100 - totalShaderkit * 100 / totalOrig).toFixed(1)}% reduction)`);
    console.log(`  Avg time per file: ${(totalShaderkitTime / files.length).toFixed(3)}ms`);

    console.log(`\n${GREEN}Comparison:${NC}`);
    console.log(`  Size: miniray produces ${((1 - totalMiniray / totalShaderkit) * 100).toFixed(1)}% smaller output`);
    console.log(`  Speed: ${totalMinirayTime < totalShaderkitTime ? 'miniray' : 'shaderkit'} is ${Math.abs(totalShaderkitTime / totalMinirayTime).toFixed(1)}x ${totalMinirayTime < totalShaderkitTime ? 'faster' : 'slower'}`);
}

main().catch(console.error);

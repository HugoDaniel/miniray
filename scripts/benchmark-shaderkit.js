#!/usr/bin/env node
/**
 * Benchmark comparing miniray (Go/WASM) vs shaderkit (JS)
 * Measures both output size and execution speed
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const { minify: shaderkitMinify } = require('shaderkit');

// Configuration
const ITERATIONS = 10;
const MINIRAY_BIN = process.env.MINIRAY_BIN || './build/miniray';
const TESTDATA_DIR = process.env.TESTDATA_DIR || 'testdata';

// Colors
const RED = '\x1b[31m';
const GREEN = '\x1b[32m';
const YELLOW = '\x1b[33m';
const BLUE = '\x1b[34m';
const NC = '\x1b[0m';

function findWgslFiles(dir) {
    const files = [];
    if (!fs.existsSync(dir)) return files;

    for (const entry of fs.readdirSync(dir)) {
        const fullPath = path.join(dir, entry);
        const stat = fs.statSync(fullPath);
        if (stat.isFile() && entry.endsWith('.wgsl')) {
            files.push(fullPath);
        }
    }
    return files.sort();
}

function measureTime(fn, iterations) {
    const times = [];
    for (let i = 0; i < iterations; i++) {
        const start = performance.now();
        fn();
        const end = performance.now();
        times.push(end - start);
    }
    return times.reduce((a, b) => a + b, 0) / times.length;
}

function benchmarkMiniray(filePath) {
    try {
        const avgTime = measureTime(() => {
            execSync(`"${MINIRAY_BIN}" "${filePath}"`, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] });
        }, ITERATIONS);

        const output = execSync(`"${MINIRAY_BIN}" "${filePath}"`, { encoding: 'utf8' });
        return { size: Buffer.byteLength(output, 'utf8'), time: avgTime, error: null };
    } catch (err) {
        return { size: null, time: null, error: err.message };
    }
}

function benchmarkShaderkit(code, mangle = true) {
    try {
        // Warm up
        shaderkitMinify(code, { mangle });

        const avgTime = measureTime(() => {
            shaderkitMinify(code, { mangle });
        }, ITERATIONS);

        const output = shaderkitMinify(code, { mangle });
        return { size: Buffer.byteLength(output, 'utf8'), time: avgTime, error: null };
    } catch (err) {
        return { size: null, time: null, error: err.message };
    }
}

function formatSize(size) {
    if (size === null) return 'ERR';
    return size.toString();
}

function formatTime(time) {
    if (time === null) return '-';
    return time.toFixed(2) + 'ms';
}

function formatPct(original, minified) {
    if (minified === null || original === 0) return '-';
    return Math.round(100 - (minified * 100 / original)) + '%';
}

function main() {
    // Check miniray exists
    if (!fs.existsSync(MINIRAY_BIN)) {
        console.error(`${RED}Error: miniray not found at ${MINIRAY_BIN}${NC}`);
        console.error('Run "make build" first');
        process.exit(1);
    }

    // Find test files
    let files = [];
    const args = process.argv.slice(2);

    if (args.length > 0) {
        files = args.filter(f => fs.existsSync(f));
    } else {
        files = findWgslFiles(TESTDATA_DIR);
        // Also check compute.toys
        const computeToysFiles = findWgslFiles(path.join(TESTDATA_DIR, 'compute.toys'));
        files = [...files, ...computeToysFiles];
    }

    if (files.length === 0) {
        console.error(`${RED}Error: No .wgsl files found${NC}`);
        process.exit(1);
    }

    // Get shaderkit version
    const shaderkitPkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'node_modules', 'shaderkit', 'package.json'), 'utf8'));

    // Print header
    console.log('');
    console.log(`${BLUE}=== WGSL Minifier Benchmark: miniray vs shaderkit ===${NC}`);
    console.log(`miniray: ${MINIRAY_BIN}`);
    console.log(`shaderkit: ${shaderkitPkg.version}`);
    console.log(`iterations: ${ITERATIONS}`);
    console.log('');
    console.log('| File                         | Original | miniray | shaderkit | miniray | shaderkit |  miniray | shaderkit |');
    console.log('|                              |    bytes |   bytes |     bytes |       % |         % |     time |      time |');
    console.log('|------------------------------|----------|---------|-----------|---------|-----------|----------|-----------|');

    let totalOrig = 0;
    let totalMiniray = 0;
    let totalShaderkit = 0;
    let miniraySuccess = 0;
    let shaderkitSuccess = 0;
    let shaderkitOrig = 0;

    for (const file of files) {
        const filename = path.basename(file);
        const code = fs.readFileSync(file, 'utf8');
        const origSize = Buffer.byteLength(code, 'utf8');

        const minirayResult = benchmarkMiniray(file);
        const shaderkitResult = benchmarkShaderkit(code, true);

        totalOrig += origSize;

        if (minirayResult.size !== null) {
            totalMiniray += minirayResult.size;
            miniraySuccess++;
        }

        if (shaderkitResult.size !== null) {
            totalShaderkit += shaderkitResult.size;
            shaderkitOrig += origSize;
            shaderkitSuccess++;
        }

        console.log(`| ${filename.padEnd(28)} | ${origSize.toString().padStart(8)} | ${formatSize(minirayResult.size).padStart(7)} | ${formatSize(shaderkitResult.size).padStart(9)} | ${formatPct(origSize, minirayResult.size).padStart(7)} | ${formatPct(origSize, shaderkitResult.size).padStart(9)} | ${formatTime(minirayResult.time).padStart(8)} | ${formatTime(shaderkitResult.time).padStart(9)} |`);
    }

    // Summary
    console.log('');
    console.log(`${BLUE}=== Summary ===${NC}`);
    console.log('');

    if (totalMiniray > 0) {
        const minirayPct = (100 - (totalMiniray * 100 / totalOrig)).toFixed(1);
        console.log(`${GREEN}miniray:${NC}`);
        console.log(`  Success: ${miniraySuccess}/${files.length} files`);
        console.log(`  Total: ${totalOrig} -> ${totalMiniray} bytes (${minirayPct}% reduction)`);
    }

    if (totalShaderkit > 0) {
        const shaderkitPct = (100 - (totalShaderkit * 100 / shaderkitOrig)).toFixed(1);
        console.log(`${GREEN}shaderkit:${NC}`);
        console.log(`  Success: ${shaderkitSuccess}/${files.length} files`);
        console.log(`  Total: ${shaderkitOrig} -> ${totalShaderkit} bytes (${shaderkitPct}% reduction on successful files)`);
    }
}

main();

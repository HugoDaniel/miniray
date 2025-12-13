#!/usr/bin/env node
/**
 * Node.js test script for miniray WASM
 * Run with: node test.js
 */

const path = require('path');
const fs = require('fs');

// Set up globals required by wasm_exec.js
globalThis.require = require;
globalThis.fs = require('fs');
globalThis.path = require('path');
globalThis.TextEncoder = require('util').TextEncoder;
globalThis.TextDecoder = require('util').TextDecoder;
globalThis.performance ??= require('perf_hooks').performance;
globalThis.crypto ??= require('crypto');

// Load wasm_exec.js to set up Go runtime
require('./wasm_exec.js');

async function main() {
    console.log('miniray WASM Node.js Test\n');

    // Initialize WASM
    const go = new Go();
    const wasmPath = path.join(__dirname, 'miniray.wasm');

    if (!fs.existsSync(wasmPath)) {
        console.error('Error: miniray.wasm not found. Run "make package-wasm" first.');
        process.exit(1);
    }

    console.log('Loading WASM from:', wasmPath);
    const wasmBuffer = fs.readFileSync(wasmPath);
    const result = await WebAssembly.instantiate(wasmBuffer, go.importObject);

    // Run the Go program (don't await - it runs in background)
    go.run(result.instance);

    // Poll for WASM initialization (regression test for version mismatch)
    // The Go runtime sets globalThis.__miniray when ready
    let initTime = 0;
    for (let i = 0; i < 50; i++) {
        if (globalThis.__miniray) {
            initTime = i * 10;
            break;
        }
        await new Promise(resolve => setTimeout(resolve, 10));
    }

    if (typeof globalThis.__miniray === 'undefined') {
        console.error('Error: __miniray not defined after WASM load');
        console.error('This usually indicates a version mismatch between wasm_exec.js and the Go compiler.');
        console.error('Run "make package-wasm" to rebuild with matching versions.');
        process.exit(1);
    }

    console.log('WASM initialized in', initTime, 'ms');
    console.log('Version:', globalThis.__miniray.version);
    console.log('');

    // Run tests
    const tests = [
        {
            name: 'Basic minification',
            input: 'const x = 1;\nconst y = 2;',
            options: { minifyWhitespace: true, minifyIdentifiers: true, minifySyntax: true },
            check: (r) => r.code.length < 25 && r.errors.length === 0
        },
        {
            name: 'Whitespace only',
            input: 'fn foo() { return 1; }',
            options: { minifyWhitespace: true, minifyIdentifiers: false },
            check: (r) => r.code.includes('foo') && r.errors.length === 0
        },
        {
            name: 'External binding preserved (default)',
            input: '@group(0) @binding(0) var<uniform> uniforms: f32;\nfn getValue() -> f32 { return uniforms * 2.0; }',
            options: { minifyWhitespace: true, minifyIdentifiers: true },
            check: (r) => r.code.includes('var<uniform> uniforms') && r.errors.length === 0
        },
        {
            name: 'External binding mangling',
            input: '@group(0) @binding(0) var<uniform> uniforms: f32;\nfn getValue() -> f32 { return uniforms * 2.0; }',
            options: { minifyWhitespace: true, minifyIdentifiers: true, mangleExternalBindings: true },
            check: (r) => !r.code.includes('uniforms') && !r.code.includes('let ') && r.errors.length === 0
        },
        {
            name: 'Keep names',
            input: 'fn myHelper() -> f32 { return 1.0; }\nfn other() -> f32 { return myHelper(); }',
            options: { minifyWhitespace: true, minifyIdentifiers: true, keepNames: ['myHelper'] },
            check: (r) => r.code.includes('myHelper') && r.errors.length === 0
        },
        {
            name: 'Complex shader',
            input: `
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
struct Uniforms { scale: f32 }
fn computeValue(index: u32) -> f32 {
    return f32(index) * uniforms.scale;
}
@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3u) {
    let value = computeValue(id.x);
}`,
            options: { minifyWhitespace: true, minifyIdentifiers: true, minifySyntax: true },
            check: (r) => r.errors.length === 0 && r.minifiedSize < r.originalSize
        },
    ];

    let passed = 0;
    let failed = 0;

    for (const test of tests) {
        try {
            const result = globalThis.__miniray.minify(test.input, test.options);
            const ok = test.check(result);

            if (ok) {
                console.log(`✓ ${test.name}`);
                passed++;
            } else {
                console.log(`✗ ${test.name}`);
                console.log(`  Output: ${result.code}`);
                if (result.errors.length > 0) {
                    console.log(`  Errors: ${result.errors.map(e => e.message).join(', ')}`);
                }
                failed++;
            }
        } catch (err) {
            console.log(`✗ ${test.name}`);
            console.log(`  Error: ${err.message}`);
            failed++;
        }
    }

    console.log(`\n${passed} passed, ${failed} failed`);

    // Show example output
    console.log('\n--- Example Output ---');
    const example = `@group(0) @binding(0) var<uniform> uniforms: f32;
fn getValue() -> f32 { return uniforms * 2.0; }`;

    console.log('Input:');
    console.log(example);

    console.log('\nWith aliasing (default):');
    const r1 = globalThis.__miniray.minify(example, { minifyWhitespace: true, minifyIdentifiers: true });
    console.log(r1.code);
    console.log(`(${r1.originalSize} -> ${r1.minifiedSize} bytes)`);

    console.log('\nWith mangleExternalBindings:');
    const r2 = globalThis.__miniray.minify(example, { minifyWhitespace: true, minifyIdentifiers: true, mangleExternalBindings: true });
    console.log(r2.code);
    console.log(`(${r2.originalSize} -> ${r2.minifiedSize} bytes)`);

    process.exit(failed > 0 ? 1 : 0);
}

main().catch(err => {
    console.error('Fatal error:', err);
    process.exit(1);
});

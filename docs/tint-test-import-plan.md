# Dawn Tint Test Import Plan for Miniray Minification Testing

## Overview

Import ~1,500 WGSL test files from Dawn Tint test suite to validate miniray minification correctness.

**Source:** `~/Development/specs-llm/repositories/dawn/test/tint/`
**Target:** `~/Development/miniray/testdata/tint/`

## Test Categories to Import

| Priority | Category | Files | Purpose |
|----------|----------|-------|---------|
| 1 | expressions/binary | ~400 | Operator precedence, type coercion |
| 2 | samples + benchmark | 14 | Real-world shader validation |
| 3 | statements | 104 | Control flow preservation |
| 4 | bug/tint | ~100 | Edge cases, regressions |
| 5 | types | 84 | Type system integrity |
| 6 | shadowing | 21 | Name resolution |
| 7 | loops | 33 | Loop semantics |
| 8 | var | 55 | Variable declarations |
| 9 | identifiers | 20 | Identifier resolution |
| 10 | out_of_order_decls | 10 | Declaration ordering |

## Test Harness Design

### Semantic Preservation Test

For each test file:
1. Parse original WGSL with miniray parser
2. Skip if parse fails (some tests may use unsupported extensions)
3. Minify with miniray (full options)
4. Parse minified output
5. Validate minified output passes semantic validation
6. Compare key properties (entry points preserved, bindings intact)

### Test File Structure

```
testdata/tint/
├── expressions/
│   └── binary/
│       ├── add/
│       ├── mul/
│       └── ...
├── statements/
├── samples/
├── benchmark/
├── bug/
├── types/
├── shadowing/
├── loops/
├── var/
├── identifiers/
└── out_of_order_decls/
```

### Test Runner

Create `internal/minifier_tests/tint_test.go`:
- Walk testdata/tint directory
- For each .wgsl file:
  - Parse → Minify → Parse again → Validate
  - Report failures with file path and error
- Track statistics (passed, failed, skipped)

## Implementation Steps

### Phase 1: Infrastructure
- [ ] Create testdata/tint directory structure
- [ ] Create test harness in internal/minifier_tests/tint_test.go
- [ ] Add helper functions for semantic comparison

### Phase 2: Import Priority 1-2 (expressions + samples)
- [ ] Copy expressions/binary tests (~400 files)
- [ ] Copy samples + benchmark tests (14 files)
- [ ] Run tests, fix any minifier issues found

### Phase 3: Import Priority 3-5 (statements, bugs, types)
- [ ] Copy statements tests (104 files)
- [ ] Copy bug/tint tests (~100 files)
- [ ] Copy types tests (84 files)
- [ ] Run tests, fix issues

### Phase 4: Import Priority 6-10 (remaining)
- [ ] Copy shadowing tests (21 files)
- [ ] Copy loops tests (33 files)
- [ ] Copy var tests (55 files)
- [ ] Copy identifiers tests (20 files)
- [ ] Copy out_of_order_decls tests (10 files)
- [ ] Final test run

### Phase 5: Subset of builtins
- [ ] Select ~100 representative builtin tests
- [ ] Copy and run

## Success Criteria

- All imported tests that parse successfully should:
  - Minify without errors
  - Parse after minification
  - Pass semantic validation after minification
  - Preserve entry point names and binding attributes

## Notes

- Some tests may use f16 or other extensions not supported by miniray - skip these
- Focus on semantic preservation, not output format matching
- Tests from bug/ category are especially valuable for edge cases

# Playground

Local scratch space for ad-hoc eval runs. **Everything inside this
directory is gitignored.** Use it to:

- Re-run committed historical baselines without dirtying git
- Experiment with new test cases before adding them to `eval-results/test-suite/`
- Save iteration-in-progress results

When a run is worth keeping, move it to `eval-results/runs/run-NNN/`
following the [naming convention](../runs/INDEX.md).

```bash
# Example: re-run the C5 suite locally
./dist/openparallax-eval \
  --suite eval-results/test-suite/c5_encoding_obfuscation.yaml \
  --config C --mode inject \
  --workspace /tmp/openparallax-eval \
  --output eval-results/playground/c5-$(date +%Y%m%d-%H%M).json
```

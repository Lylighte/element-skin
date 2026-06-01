# Union Branch Strategy

## Overview
The `union` branch is a standalone feature branch for the Union federation system. It is maintained separately from `main` and `dev` and will NEVER be merged back into them.

## Manual CI Trigger
To trigger a CI build for the union branch:

1. **GitHub Web UI**: Actions tab → Select workflow → "Run workflow" dropdown → Choose `union` branch → Click "Run workflow"
2. **GitHub CLI**: `gh workflow run <workflow-name> --ref union`
3. **Tag Push**: If workflows trigger on tags: `git tag union-v2.3.0-1 && git push origin union-v2.3.0-1`

## Branch Maintenance
To update the union branch with latest changes from main:

```bash
git checkout union
git merge main
# Resolve any conflicts (usually trivial since union files are isolated)
git push origin union
```

**Never merge `union` into `main` or `dev`.**

## Database Migration
The first time the union branch application starts, the database is automatically updated:

- `union_nonces` table created (replay protection for UnionHostVerify)
- `union_*` settings added with safe defaults
- All migrations use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT (key) DO NOTHING` — safe to run on existing databases

No manual migration scripts are needed.

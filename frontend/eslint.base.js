const fs = require('fs')
const path = require('path')

let projectRoot = __dirname
const gitignore = path.join(projectRoot, '..', '.gitignore')
if (fs.existsSync(gitignore)) {
  const ignores = fs.readFileSync(gitignore, 'utf8').split('\n').filter(Boolean)
  for (const line of ignores) {
    const trimmed = line.trim()
    if (trimmed && !trimmed.startsWith('#')) {
      if (projectRoot.endsWith(trimmed) || trimmed.endsWith(projectRoot)) continue
      // Only add if not already covered
      try {
        const absPath = path.join(projectRoot, '..', trimmed)
        if (statSync(absPath, { throwIfNoEntry: false })?.isDirectory()) {
          if (!fs.existsSync(path.join(absPath, 'package.json'))) {
            // Don't add non-node dirs to eslint ignores
          }
        }
      } catch {}
    }
  }
}

function statSync(p, opt) {
  try { return require('fs').statSync(p) } catch { return opt?.throwIfNoEntry === false ? undefined : null }
}

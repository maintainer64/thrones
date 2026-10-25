// Read-only syntax-aware coverage check: every t() key must be translated.
import { readFileSync, readdirSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../", import.meta.url));
const ts = createRequire(join(root, "web/package.json"))("typescript");
const dictionaries = ["ru", "en"].map((language) =>
  JSON.parse(
    readFileSync(join(root, `web/src/locales/${language}.json`), "utf8"),
  ),
);
const errors = [];
let count = 0;
function scan(directory) {
  for (const item of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, item.name);
    if (item.isDirectory() && !["i18n", "locales"].includes(item.name))
      scan(path);
    else if (
      item.isFile() &&
      /\.tsx?$/.test(path) &&
      !path.includes(".test.")
    ) {
      const source = ts.createSourceFile(
        path,
        readFileSync(path, "utf8"),
        ts.ScriptTarget.Latest,
        true,
      );
      function visit(node) {
        if (ts.isImportDeclaration(node)) return;
        // Canonical Russian place-name table has a separate English source.
        if (
          ts.isVariableDeclaration(node) &&
          node.name.getText(source) === "regions"
        )
          return;
        if (
          ts.isCallExpression(node) &&
          ts.isIdentifier(node.expression) &&
          node.expression.text === "t"
        ) {
          count++;
          const key = node.arguments[0];
          if (!key || !ts.isStringLiteral(key))
            errors.push(`${path}: t() requires a literal key`);
          else {
            for (const [index, dictionary] of dictionaries.entries())
              if (!Object.hasOwn(dictionary, key.text))
                errors.push(
                  `${path}: missing ${index === 0 ? "ru" : "en"} key ${JSON.stringify(key.text)}`,
                );
          }
          node.arguments.slice(1).forEach(visit);
          return;
        }
        if (
          (ts.isStringLiteral(node) ||
            ts.isNoSubstitutionTemplateLiteral(node) ||
            ts.isJsxText(node)) &&
          /[А-Яа-яЁё]/.test(node.text)
        )
          errors.push(
            `${path}: untranslated literal ${JSON.stringify(node.text.trim())}`,
          );
        ts.forEachChild(node, visit);
      }
      visit(source);
    }
  }
}
scan(join(root, "web/src"));
if (errors.length) {
  console.error(errors.join("\n"));
  process.exitCode = 1;
} else console.log(`RU/EN coverage OK: ${count} translation calls.`);

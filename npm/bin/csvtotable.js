#!/usr/bin/env node

const { spawnSync } = require("node:child_process");

const packages = {
  "darwin-arm64": "@vividvilla/csvtotable-darwin-arm64",
  "darwin-x64": "@vividvilla/csvtotable-darwin-x64",
  "linux-arm64": "@vividvilla/csvtotable-linux-arm64",
  "linux-x64": "@vividvilla/csvtotable-linux-x64",
  "win32-x64": "@vividvilla/csvtotable-win32-x64",
};
const platform = `${process.platform}-${process.arch}`;
const packageName = packages[platform];

if (!packageName) {
  console.error(`csvtotable does not provide a binary for ${platform}`);
  process.exit(1);
}

let executable;
try {
  executable = require.resolve(
    `${packageName}/bin/csvtotable${process.platform === "win32" ? ".exe" : ""}`,
  );
} catch {
  console.error(`csvtotable's optional package for ${platform} was not installed`);
  process.exit(1);
}

const result = spawnSync(executable, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 1);

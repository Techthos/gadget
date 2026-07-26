// Builds the gomukit runtime bundle (JS + CSS) into internal/assets/dist,
// where Go picks it up via go:embed. Output must be deterministic for a given
// esbuild version (pinned exactly in package.json) so `make verify-dist` works.
import esbuild from "esbuild";

const outdir = "../internal/assets/dist";

await esbuild.build({
  entryPoints: ["src/index.ts"],
  bundle: true,
  format: "iife",
  target: "es2020",
  minify: true,
  legalComments: "none",
  outfile: `${outdir}/gomukit.js`,
});

await esbuild.build({
  entryPoints: ["css/index.css"],
  bundle: true,
  minify: true,
  legalComments: "none",
  outfile: `${outdir}/gomukit.css`,
});

console.log("built internal/assets/dist/gomukit.{js,css}");

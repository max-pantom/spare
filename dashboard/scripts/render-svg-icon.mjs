import { readFile } from "node:fs/promises";
import sharp from "sharp";

const [sourcePath, outputPath, requestedSize = "1024"] = process.argv.slice(2);
const size = Number.parseInt(requestedSize, 10);

if (!sourcePath || !outputPath || !Number.isInteger(size) || size < 1) {
  console.error(
    "Usage: node scripts/render-svg-icon.mjs <source.svg> <output.png> [size]",
  );
  process.exit(1);
}

const svg = await readFile(sourcePath, "utf8");

await sharp(Buffer.from(svg), { density: 192 })
  .resize(size, size, { fit: "fill" })
  .png()
  .toFile(outputPath);

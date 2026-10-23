// Reuse local game assets without modifying or uploading the source export.
import { copyFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = process.argv[2] || "/Users/gubanov/Desktop/Resources.out/AssetRipper_export_20261018_045740/Assets/Texture2D";
await mkdir(path.join(root, "local-assets/ui"), { recursive: true });
await copyFile(path.join(source, "S_icon_BattleShield_Strength.png"), path.join(root, "local-assets/ui/strength-shield.png"));
console.log("Local strength shield imported. Original export unchanged.");

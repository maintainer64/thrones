"""Local-only, read-only import from an owned installation. Never modifies the app.

Run: uv run --with UnityPy python scripts/import-installed-game.py
Output is ignored by Git/Docker and mounted read-only into the local container.
"""
import argparse
import json
import re
from pathlib import Path

import UnityPy

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--app", type=Path, default=Path("/Applications/A Game of Thrones The Board Game.app"))
args = parser.parse_args()
source = args.app / "Contents/Resources/Data"
if not source.is_dir():
    raise SystemExit(f"Game data not found: {source}")
output = Path(__file__).resolve().parent.parent / "local-assets"
output.mkdir(exist_ok=True)
manifest = {"localOnly": True, "textures": {}, "sounds": {}, "tutorialKeys": []}
bundles = sorted((source / "StreamingAssets/Bundles").glob("b_*"))
for bundle in bundles:
    environment = UnityPy.load(str(bundle))
    count = 0
    for obj in environment.objects:
        if obj.type.name != "Texture2D":
            continue
        texture = obj.read()
        name = texture.m_Name
        # Only the base game's runtime art is needed. Original expansion bundles
        # stay in the installed game / neighbouring workbench, not the web root.
        if re.match(r"T_crd_(2_|3_|Wildling_|westeros_|vassal)", name):
            continue
        if not re.fullmatch(r"[A-Za-z0-9_-]+", name):
            continue
        filename = name + ".png"
        texture.image.save(output / filename)
        manifest["textures"][name] = "/local-assets/" + filename
        count += 1
    print(f"{bundle.name}: {count} textures", flush=True)
effects = {"T_fx_OrderRing", "T_fx_CircleGlow", "T_fx_PointerGlow", "T_fx_DoubleRing", "T_fx_AreaGlow"}
sounds = {"ui-battle-show", "ui-battle-cardreveal2", "ui-battle-victorystinger", "unit-knight-running-loop", "ui-playmat-alert-localplayeraction", "ui-button2"}
environment = UnityPy.load(str(source / "resources.assets"))
for obj in environment.objects:
    if obj.type.name not in ("Texture2D", "AudioClip"):
        continue
    data = obj.read()
    name = data.m_Name
    if obj.type.name == "Texture2D" and name in effects:
        data.image.save(output / (name + ".png"))
        manifest["textures"][name] = "/local-assets/" + name + ".png"
    elif obj.type.name == "AudioClip" and name in sounds:
        try:
            for index, (_, contents) in enumerate(data.samples.items()):
                filename = name + f"-{index}.wav"
                (output / filename).write_bytes(contents)
                manifest["sounds"][name] = "/local-assets/" + filename
        except Exception as error:
            print(f"Audio {name} not exported: {error}", flush=True)
localization = source / "StreamingAssets/Localization/en_US.txt"
if localization.exists():
    text = localization.read_text(encoding="utf-8-sig")
    manifest["tutorialKeys"] = sorted(set(re.findall(r"tutorial\.[\w.]+", text)))
    # Keep original tutorial text locally as a research reference, never ship it in public assets.
    research = output.parent.parent / (output.parent.name + "-workbench") / "unused-imports"
    research.mkdir(parents=True, exist_ok=True)
    (research / "tutorial-source-en.txt").write_text(text, encoding="utf-8")
(output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
print(f"Imported {len(manifest['textures'])} textures into {output}; installed game unchanged.")

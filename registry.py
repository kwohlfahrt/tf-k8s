#!/usr/bin/env -S uv run --script
#
# /// script
# dependencies = [
#   "aiohttp",
# ]
# ///

import asyncio
from collections.abc import AsyncIterator
from dataclasses import dataclass
import json
from pathlib import Path
from tempfile import TemporaryDirectory
from typing import Self

import aiohttp

user = "kwohlfahrt"
repo = "tf-k8s"


class Gpg:
    def __init__(self, key_name: str, dir: Path):
        self.key_name = key_name
        self.dir = dir

    async def armor(self) -> str:
        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={self.dir}",
            "--armor",
            "--export",
            self.key_name,
            stdout=asyncio.subprocess.PIPE,
        )
        stdout, _ = await proc.communicate()
        assert proc.returncode == 0
        return stdout.decode()

    async def key_id(self) -> str:
        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={self.dir}",
            "--keyid-format=long",
            "--list-keys",
            "--with-colons",
            self.key_name,
            stdout=asyncio.subprocess.PIPE,
        )
        stdout, _ = await proc.communicate()
        assert proc.returncode == 0
        for line in stdout.decode().splitlines():
            parts = line.split(":")
            if parts[0] == "pub":
                return parts[4]

    @classmethod
    async def new(cls, key_name: str, dir: Path):
        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={dir}",
            "--batch",
            "--passphrase=",
            "--quick-generate-key",
            key_name,
            "ed25519",
            "sign",
            "never",
        )
        await proc.wait()
        assert proc.returncode == 0
        return cls(key_name, dir)


@dataclass
class Platform:
    os: str
    arch: str


@dataclass
class Release:
    provider: str
    version: str
    protocols: list[str]
    platforms: list[Platform]
    assets: dict[str, str]
    file_hashes: dict[str, str]

    async def write_files(self, base: Path, gpg: tuple[str, str]):
        download = base / self.provider / self.version / "download"
        for platform in self.platforms:
            path = download / platform.os / platform.arch
            path.parent.mkdir(parents=True, exist_ok=True)
            filename = "terraform-provider-k8s-{self.provider}_{self.version}_{platform.os}_{platform.arch}.zip"
            path.write_text(
                json.dumps(
                    {
                        "protocols": self.protocols,
                        "os": platform.os,
                        "arch": platform.arch,
                        "filename": filename,
                        "download_url": self.assets[filename],
                        "shasums_url": self.assets["SHA2556SUMS"],
                        "shasums_signature_url": "",
                        "shasum": self.file_hashes[filename],
                        "signing_keys": {
                            "gpg_public_keys": [
                                {"key_id": gpg[0], "ascii_armor": gpg[1]}
                            ]
                        },
                    }
                )
            )

    @classmethod
    async def from_json(cls, sess: aiohttp.ClientSession, json) -> Self:
        provider, _ = json["name"].split(maxsplit=1)
        assets = {a["name"]: a["browser_download_url"] for a in json["assets"]}
        async with sess.get(assets["version.json"]) as r:
            r.raise_for_status()
            version = await r.json()

        async with sess.get(assets["SHA256SUMS"]) as r:
            r.raise_for_status()
            shasums = await r.text()

        file_hashes = {
            filename: shasum
            for shasum, filename in map(str.split, shasums.splitlines())
        }

        return cls(
            provider=provider,
            version=version["version"],
            protocols=version["protocols"],
            platforms=[Platform(**p) for p in version["platforms"]],
            assets=assets,
            file_hashes=file_hashes,
        )


async def get_releases(sess: aiohttp.ClientSession) -> AsyncIterator[Release]:
    headers = {
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }

    page = f"https://api.github.com/repos/{user}/{repo}/releases"
    while page:
        async with sess.get(page, headers=headers) as r:
            r.raise_for_status()
            page = r.links.get("next", {}).get("url", None)
            for item in await r.json():
                if any(a["name"] == "version.json" for a in item["assets"]):
                    yield Release.from_json(sess, item)


async def main():
    base_path = Path(".")

    with TemporaryDirectory() as d:
        key = await Gpg.new("GitHub Actions Bot", dir=d)
        key_id, key_armor = await asyncio.gather(key.key_id(), key.armor())
        async with aiohttp.ClientSession() as sess:
            await asyncio.gather(
                *[
                    r.write(base_path, (key_id, key_armor))
                    async for r in get_releases(sess)
                ]
            )

        path = base_path / ".well-known" / "terraform.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write(
            json.dumps(
                {
                    "providers.v1": f"https://{user}.github.io/{repo}/registry/providers/v1/"
                }
            )
        )


if __name__ == "__main__":
    asyncio.run(main())

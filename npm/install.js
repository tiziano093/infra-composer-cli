#!/usr/bin/env node
'use strict';

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const VERSION = require('./package.json').version;
const REPO = 'tiziano093/infra-composer-cli';

const PLATFORM_MAP = {
  linux: 'linux',
  darwin: 'darwin',
  win32: 'windows',
};

const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64',
};

function getPlatform() {
  const os = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];
  if (!os || !arch) {
    throw new Error(`Unsupported platform: ${process.platform}/${process.arch}`);
  }
  return { os, arch };
}

function getBinaryName(os) {
  return os === 'windows' ? 'infra-composer.exe' : 'infra-composer';
}

function getDownloadUrl(os, arch) {
  const ext = os === 'windows' ? 'zip' : 'tar.gz';
  const archive = `infra-composer_v${VERSION}_${os}_${arch}.${ext}`;
  return `https://github.com/${REPO}/releases/download/v${VERSION}/${archive}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    function get(u) {
      https.get(u, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          return get(res.headers.location);
        }
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode} for ${u}`));
          return;
        }
        res.pipe(file);
        file.on('finish', () => file.close(resolve));
      }).on('error', reject);
    }
    get(url);
  });
}

async function install() {
  const { os, arch } = getPlatform();
  const url = getDownloadUrl(os, arch);
  const binDir = path.join(__dirname, 'bin');
  const binaryName = getBinaryName(os);
  const binaryPath = path.join(binDir, binaryName);

  if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });

  const tmpArchive = path.join(binDir, 'archive.tmp');
  console.log(`Downloading infra-composer v${VERSION} for ${os}/${arch}...`);
  await download(url, tmpArchive);

  if (os === 'windows') {
    execSync(`powershell -Command "Expand-Archive -Path '${tmpArchive}' -DestinationPath '${binDir}' -Force"`);
  } else {
    execSync(`tar -xzf '${tmpArchive}' -C '${binDir}' '${binaryName}'`);
  }

  fs.unlinkSync(tmpArchive);
  fs.chmodSync(binaryPath, 0o755);
  console.log(`Installed to ${binaryPath}`);
}

install().catch((err) => {
  console.error('Installation failed:', err.message);
  process.exit(1);
});

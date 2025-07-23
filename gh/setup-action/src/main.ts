import * as core from '@actions/core'
import * as github from '@actions/github'
import { RestEndpointMethodTypes } from '@octokit/plugin-rest-endpoint-methods'
import { execSync } from 'child_process'
import * as fs from 'fs'
import { createWriteStream } from 'fs'
import * as os from 'os'
import * as path from 'path'
import { pipeline } from 'stream'
import { promisify } from 'util'

const streamPipeline = promisify(pipeline)

type LatestReleaseResponse =
  RestEndpointMethodTypes['repos']['getLatestRelease']['response']['data']

const platformMap: Record<string, string> = {
  linux: 'linux',
  darwin: 'darwin',
  win32: 'windows'
}

const archMap: Record<string, string> = {
  x64: 'amd64',
  arm64: 'arm64'
}

async function downloadToFile(url: string, dest: string): Promise<void> {
  const res = await fetch(url)

  if (!res.ok || !res.body) {
    throw new Error(
      `Failed to download ${url} to ${dest}: ${res.status} ${res.statusText}`
    )
  }

  await streamPipeline(res.body, createWriteStream(dest))
}

async function getLatestVersion(): Promise<string> {
  const { owner, repo } = github.context.repo
  const url = `https://api.github.com/repos/${owner}/${repo}/releases/latest`

  const res = await fetch(url, {
    headers: { 'User-Agent': 'mimosa-downloader' } // required by GitHub API
  })

  if (!res.ok) {
    throw new Error(
      `Failed to fetch latest release: ${res.status} ${res.statusText}`
    )
  }

  const json = (await res.json()) as LatestReleaseResponse
  return json.tag_name
}

export async function run(): Promise<void> {
  try {
    let version: string = core.getInput('version')
    const toolFile: string = core.getInput('tool-file')

    if (version === 'latest' || (!toolFile && !version)) {
      version = await getLatestVersion()
    }

    if (toolFile) {
      let mimosaLine = fs
        .readFileSync(toolFile)
        .toString()
        .split('\n')
        .find((s) => s.includes('mimosa'))
        ?.trim()
      if (!mimosaLine) {
        core.setFailed(`Tools file ${toolFile} does not contain mimosa`)
        return
      }

      version = mimosaLine.replaceAll('mimosa', '')
    }

    version = version.replaceAll('v', '').trim()

    if (!version) {
      core.setFailed(`Invalid version ${version}`)
      return
    }

    const runner = {
      os: platformMap[process.platform],
      arch: archMap[process.arch]
    }

    if (!runner.os || !runner.arch) {
      // mimosa not supported in this platform
      core.setFailed(
        `Unsupported platform or architecture: ${process.platform} ${process.arch}`
      )
      return
    }

    const downloadUrl = `https://github.com/hytromo/mimosa/releases/download/v${version}/mimosa_${version}_${runner.os}_${runner.arch}.tar.gz`

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mimosa-'))
    const archivePath = path.join(tmpDir, 'mimosa.tar.gz')

    await downloadToFile(downloadUrl, archivePath)

    execSync(`tar -xzf ${archivePath} -C ${tmpDir}`)

    const mimosaPath = path.join(tmpDir, 'mimosa')
    const targetPath = '/usr/local/bin/mimosa'
    fs.copyFileSync(mimosaPath, targetPath)
    fs.chmodSync(targetPath, 0o755)

    fs.rmSync(tmpDir, { recursive: true, force: true })

    console.log(`Installed mimosa version ${version} at ${targetPath}`)

    core.setOutput('path', targetPath)
  } catch (error) {
    // Fail the workflow run if an error occurs
    if (error instanceof Error) core.setFailed(error.message)
  }
}

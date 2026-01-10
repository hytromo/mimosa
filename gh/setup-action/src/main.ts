import * as core from '@actions/core'
import * as github from '@actions/github'
import toolCache from '@actions/tool-cache'
import type { RestEndpointMethodTypes } from '@octokit/plugin-rest-endpoint-methods'
import { execSync } from 'node:child_process'
import * as fs from 'node:fs'
import * as path from 'node:path'

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

async function getLatestVersion(): Promise<string> {
  const url = `https://api.github.com/repos/hytromo/mimosa/releases/latest`

  const res = await fetch(url, {
    headers: { 'User-Agent': 'mimosa-downloader' } // required by GitHub API
  })

  if (!res.ok) {
    throw new Error(
      `Failed to fetch latest release: ${res.status} ${res.statusText}`
    )
  }

  const json = (await res.json()) as LatestReleaseResponse
  return json.tag_name.replaceAll('v', '').trim()
}

export async function run(): Promise<void> {
  try {
    let version: string = core.getInput('version')
    const toolFile: string = core.getInput('tool-file')

    if (toolFile) {
      const mimosaLine = fs
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

    if (version === 'latest' || (!toolFile && !version)) {
      version = await getLatestVersion()
      console.log(`Using latest version: ${version}`)
    }

    if (!version) {
      core.setFailed(`Invalid version ${version}`)
      return
    }

    const runner = {
      os: platformMap[process.platform],
      arch: archMap[process.arch]
    }

    if (!runner.os || !runner.arch) {
      // mimosa not supported in this platform!
      core.setFailed(
        `Unsupported platform or architecture: ${process.platform} ${process.arch}`
      )
      return
    }

    // let's see if we find the tool in the cache first
    const cachedToolPath = toolCache.find('mimosa', version)
    let binaryPath = ''
    const binaryFileName = runner.os === 'windows' ? 'mimosa.exe' : 'mimosa'

    if (cachedToolPath) {
      core.addPath(cachedToolPath)
      binaryPath = path.join(cachedToolPath, binaryFileName)
    } else {
      const downloadUrl = `https://github.com/hytromo/mimosa/releases/download/v${version}/mimosa_${version}_${runner.os}_${runner.arch}.tar.gz`

      core.info(`Downloading ${downloadUrl}`)
      const downloadPath = await toolCache.downloadTool(downloadUrl)

      const extractPath = await toolCache.extractTar(downloadPath)
      core.info(`Extracted to ${extractPath}`)

      const binaryPathInExtract = path.join(extractPath, binaryFileName)
      fs.chmodSync(binaryPathInExtract, 0o755)

      const cachedDir = await toolCache.cacheFile(
        binaryPathInExtract,
        binaryFileName,
        'mimosa',
        version
      )
      core.addPath(cachedDir)
      core.info(`Cached at ${cachedDir} - which is also added to PATH`)

      binaryPath = path.join(cachedDir, binaryFileName)
    }

    core.setOutput(
      'cache-path',
      execSync(`"${binaryPath}" cache --show`).toString().trim()
    )

    core.setOutput('binary-path', binaryPath)

    console.log(`Installed mimosa version ${version} at ${binaryPath}`)
  } catch (error) {
    // Fail the workflow run if an error occurs
    if (error instanceof Error) core.setFailed(error.message)
  }
}

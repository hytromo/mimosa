import * as core from '@actions/core'
import * as github from '@actions/github'
import { Octokit } from '@octokit/rest'
import { execSync } from 'node:child_process'
import fs, { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'

export async function run(): Promise<void> {
  try {
    const githubToken: string = core.getInput('github-token')
    const repoVariableName: string = core.getInput('variable-name')
    const maxLength: number = parseInt(core.getInput('max-length'), 10)

    if (Number.isNaN(maxLength)) {
      core.warning(`max-length is invalid: ${maxLength}`)
      return
    }

    if (!githubToken) {
      core.warning('github-token is required')
      return
    }

    if (!repoVariableName) {
      core.warning('variable-name is required')
      return
    }

    // initialize octokit client
    const octokit = new Octokit({
      auth: githubToken
    })
    const context = github.context

    const mimosaEnv = { ...process.env }

    // read the existing variable if it exists
    try {
      const response = await octokit.rest.actions.getRepoVariable({
        owner: context.repo.owner,
        repo: context.repo.repo,
        name: repoVariableName
      })
      const existingMimosaCacheEnv = (response.data.value || '')?.trim()
      console.log(
        `Existing variable ${repoVariableName} found with value length: ${existingMimosaCacheEnv.length}`
      )
      if (existingMimosaCacheEnv) {
        // set the existing variable value to the environment
        mimosaEnv['MIMOSA_CACHE'] = existingMimosaCacheEnv
      }
    } catch (error: any) {
      // ignore any error
      if (error.status !== 404) {
        console.error(
          `Failed to get existing variable ${repoVariableName}:`,
          error
        )
      }
    }

    const tmpDir = mkdtempSync(path.join(tmpdir(), 'mimosa-'))
    const envTmpFilePath = path.join(tmpDir, 'tempfile.txt')

    execSync(`mimosa cache --export-to "${envTmpFilePath}"`, {
      env: mimosaEnv
    })

    let newMimosaCacheEnv = fs.readFileSync(envTmpFilePath, 'utf-8').trim()

    fs.rmSync(tmpDir, { recursive: true, force: true })

    if (newMimosaCacheEnv.length > maxLength) {
      // we need to remove as many lines from the end of newMimosaCacheEnv as needed in order to fit the max length
      // this is due to Github Actions variable length limits or stricter limits imposed by the user
      const lines = newMimosaCacheEnv.split('\n')
      const charactersToRemove = newMimosaCacheEnv.length - maxLength

      console.log(
        `Variable length exceeds max length by ${charactersToRemove} characters`
      )

      let charactersRemoved = 0
      let linesToRemove = 0
      for (let i = lines.length - 1; i >= 0; i--) {
        linesToRemove++

        charactersRemoved += lines[i].length

        if (i < lines.length - 1) {
          charactersRemoved++ // account for the newline character
        }

        if (charactersRemoved >= charactersToRemove) {
          break
        }
      }

      newMimosaCacheEnv = lines
        .slice(0, lines.length - linesToRemove)
        .join('\n')
        .trim()

      console.log(
        `Trimmed cache to fit max length: ${newMimosaCacheEnv.length} characters by removing ${linesToRemove} entries`
      )
    }

    if (mimosaEnv['MIMOSA_CACHE'] === newMimosaCacheEnv) {
      console.log(
        `Mimosa cache is already up to date with the latest version of variable ${repoVariableName} - skipping update.`
      )
      return
    }

    try {
      // mimosa env variable changed
      await octokit.rest.actions.updateRepoVariable({
        owner: context.repo.owner,
        repo: context.repo.repo,
        name: repoVariableName,
        value: newMimosaCacheEnv
      })
      console.log(
        `Updated variable ${repoVariableName} with new cache value with ${newMimosaCacheEnv.split('\n').length} entries in total.`
      )
    } catch (error: any) {
      if (error.status === 404) {
        // If the variable does not exist, create it
        await octokit.rest.actions.createRepoVariable({
          owner: context.repo.owner,
          repo: context.repo.repo,
          name: repoVariableName,
          value: newMimosaCacheEnv
        })
        console.log(
          `Created variable ${repoVariableName} with new cache value with ${newMimosaCacheEnv.split('\n').length} entries in total.`
        )
      } else {
        core.warning(
          `Failed to update variable ${repoVariableName}: ${error?.status} - ${error?.message}`
        )
      }
    }
  } catch (error) {
    // Fail the workflow run if an error occurs
    if (error instanceof Error) {
      core.warning(`Mimosa cache save error: ${error.message}`)
    }
  }
}

* This project is configured using [mise-en-place](https://mise.jdx.dev/getting-started.html) to install other tools while keeping the host system clean, and run tasks like unit tests. Mise will need to be installed.

* Clone the repo: `git clone https://github.com/hytromo/mimosa.git`

* Initialize the whole project without polluting your global environment: `mise run init:local`

Start hacking! The pre-commit hooks will help ensuring that the github actions are bundled or that the go code does not have code smells. Have a look at `.pre-commit-config.yaml` for details

See `mise tasks` for all the possible tasks, e.g. `mise run tests:unit`, `mise run tests:integration`, `mise run tests:cleanup`

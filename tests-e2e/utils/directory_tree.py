import json
import os
import sys


def get_directory_paths(directory):
  paths = []
  for item in os.listdir(directory):
    item_path = os.path.join(directory, item)

    if os.path.isfile(item_path):
      paths.append(os.path.relpath(item_path, directory))
    elif os.path.isdir(item_path):
      paths.append(os.path.relpath(item_path, directory))
      for sub_path in get_directory_paths(item_path):
        paths.append(os.path.join(item, sub_path))

  return paths


def main():
  exitCode = 0 if sys.argv[1] in ["get", "compare"] else 1

  if sys.argv[1] == "get":
    print(json.dumps(get_directory_paths(sys.argv[2])))
  elif sys.argv[1] == "compare":
    exitCode = (
      0
      if set(get_directory_paths(sys.argv[2])) == set(json.loads(sys.stdin.read()))
      else 1
    )

  sys.exit(exitCode)


if __name__ == "__main__":
  main()

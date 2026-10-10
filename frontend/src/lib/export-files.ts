import type { ExportFile } from '@/lib/api/imports'

const isCsv = (path: string) => path.toLowerCase().endsWith('.csv')

/** CSV files of a folder picked through an `<input webkitdirectory>`. */
export function filesFromFolderInput(files: FileList): ExportFile[] {
  return Array.from(files, (file) => ({ file, path: file.webkitRelativePath })).filter(({ path }) =>
    isCsv(path),
  )
}

/** CSV files of a dropped folder, read recursively. */
export async function filesFromDroppedFolder(
  directory: FileSystemDirectoryEntry,
): Promise<ExportFile[]> {
  const files: ExportFile[] = []

  for (const entry of await readEntries(directory)) {
    if (entry.isDirectory) {
      files.push(...(await filesFromDroppedFolder(entry as FileSystemDirectoryEntry)))
    } else if (isCsv(entry.name)) {
      const file = await new Promise<File>((resolve, reject) =>
        (entry as FileSystemFileEntry).file(resolve, reject),
      )
      // fullPath starts with a slash: "/letterboxd-user-2026-10-10/ratings.csv".
      files.push({ file, path: entry.fullPath.replace(/^\//, '') })
    }
  }

  return files
}

/** readEntries returns results in batches: keep calling it until it comes back empty. */
async function readEntries(directory: FileSystemDirectoryEntry): Promise<FileSystemEntry[]> {
  const reader = directory.createReader()
  const entries: FileSystemEntry[] = []

  while (true) {
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) =>
      reader.readEntries(resolve, reject),
    )
    if (batch.length === 0) return entries
    entries.push(...batch)
  }
}

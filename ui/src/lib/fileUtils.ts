import JSZip from 'jszip';
import pako from 'pako';

export async function extractQMDsFromZip(file: File): Promise<File[]> {
  const zip = await JSZip.loadAsync(file);
  const qmdFiles: File[] = [];

  for (const [filename, zipEntry] of Object.entries(zip.files)) {
    if (!zipEntry.dir && filename.toLowerCase().endsWith('.qmd')) {
      const blob = await zipEntry.async('blob');
      const newFile = new File([blob], filename, {
        type: 'text/plain'
      });
      qmdFiles.push(newFile);
    }
  }

  return qmdFiles;
}

export async function extractQMDsFromTarGz(file: File): Promise<File[]> {
  const qmdFiles: File[] = [];

  const arrayBuffer = await file.arrayBuffer();
  const uint8Array = new Uint8Array(arrayBuffer);

  const decompressed = pako.ungzip(uint8Array);

  let offset = 0;
  while (offset < decompressed.length) {
    const header = decompressed.slice(offset, offset + 512);

    const nameBytes = header.slice(0, 100);
    const nameEnd = nameBytes.indexOf(0);
    const filename = new TextDecoder().decode(nameBytes.slice(0, nameEnd > 0 ? nameEnd : 100)).trim();

    const sizeBytes = header.slice(124, 136);
    const sizeStr = new TextDecoder().decode(sizeBytes).trim().replace(/\0/g, '');
    const fileSize = parseInt(sizeStr, 8) || 0;

    const fileType = String.fromCharCode(header[156]);

    offset += 512;

    if (filename && fileSize > 0 && (fileType === '0' || fileType === '\0') && filename.toLowerCase().endsWith('.qmd')) {
      const fileData = decompressed.slice(offset, offset + fileSize);
      const blob = new Blob([fileData], { type: 'text/plain' });
      const qmdFile = new File([blob], filename, {
        type: 'text/plain'
      });
      qmdFiles.push(qmdFile);
    }

    offset += Math.ceil(fileSize / 512) * 512;

    if (offset + 1024 <= decompressed.length) {
      const nextBlock = decompressed.slice(offset, offset + 1024);
      if (nextBlock.every((byte: number) => byte === 0)) {
        break;
      }
    }
  }

  return qmdFiles;
}

export async function extractQMDsFromFolder(
  entry: FileSystemEntry
): Promise<File[]> {
  const qmdFiles: File[] = [];

  async function traverseDirectory(dirEntry: FileSystemDirectoryEntry, basePath: string = '') {
    const reader = dirEntry.createReader();

    const allEntries: FileSystemEntry[] = [];
    let batch: FileSystemEntry[];

    do {
      batch = await new Promise<FileSystemEntry[]>((resolve) => {
        reader.readEntries(resolve);
      });
      allEntries.push(...batch);
    } while (batch.length > 0);

    for (const entry of allEntries) {
      const relativePath = basePath ? `${basePath}/${entry.name}` : entry.name;

      if (entry.isFile) {
        const fileEntry = entry as FileSystemFileEntry;
        if (fileEntry.name.toLowerCase().endsWith('.qmd')) {
          const file = await new Promise<File>((resolve) => {
            fileEntry.file(resolve);
          });
          const fileWithPath = new File([file], relativePath, {
            type: 'text/plain'
          });
          qmdFiles.push(fileWithPath);
        }
      } else if (entry.isDirectory) {
        await traverseDirectory(entry as FileSystemDirectoryEntry, relativePath);
      }
    }
  }

  if (entry.isDirectory) {
    await traverseDirectory(entry as FileSystemDirectoryEntry);
  } else if (entry.isFile && entry.name.toLowerCase().endsWith('.qmd')) {
    const fileEntry = entry as FileSystemFileEntry;
    const file = await new Promise<File>((resolve) => {
      fileEntry.file(resolve);
    });
    qmdFiles.push(file);
  }

  return qmdFiles;
}

export function sortFilesLexicographically(files: File[]): File[] {
  return [...files].sort((a, b) => a.name.localeCompare(b.name));
}

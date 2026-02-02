'use client'

import { useCallback, useEffect, useState, useRef } from 'react'
import { Upload, File, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { extractQMDsFromZip, extractQMDsFromTarGz, extractQMDsFromFolder, sortFilesLexicographically } from '@/lib/fileUtils'

interface FileDropzoneProps {
  onUpload: (files: File[], paths: string[]) => void
  disabled?: boolean
  onError?: (message: string) => void
}

interface FileEntry {
  file: File
  path: string
}

const ACCEPT_CONFIG = {
  'text/plain': ['.qmd'],
  'application/zip': ['.zip'],
  'application/x-gzip': ['.tar.gz', '.gz'],
}

export function FileDropzone({
  onUpload,
  disabled = false,
  onError,
}: FileDropzoneProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [dragActive, setDragActive] = useState(false)
  const [files, setFiles] = useState<FileEntry[]>([])

  const processFiles = useCallback(
    async (inputFiles: File[]) => {
      const allQMDFiles: File[] = []

      for (const file of inputFiles) {
        if (file.name.toLowerCase().endsWith('.zip')) {
          try {
            const extracted = await extractQMDsFromZip(file)
            allQMDFiles.push(...extracted)
          } catch (err) {
            if (onError) {
              onError(`Failed to extract ${file.name}: ${err}`)
            }
          }
        } else if (file.name.toLowerCase().endsWith('.tar.gz') || file.name.toLowerCase().endsWith('.tgz')) {
          try {
            const extracted = await extractQMDsFromTarGz(file)
            allQMDFiles.push(...extracted)
          } catch (err) {
            if (onError) {
              onError(`Failed to extract ${file.name}: ${err}`)
            }
          }
        } else if (file.name.toLowerCase().endsWith('.qmd')) {
          allQMDFiles.push(file)
        } else {
          if (onError) {
            onError(`Unsupported file type: ${file.name}`)
          }
        }
      }

      if (allQMDFiles.length === 0) {
        if (onError) {
          onError('No .qmd files found')
        }
        return
      }

      const sortedFiles = sortFilesLexicographically(allQMDFiles)
      const newEntries: FileEntry[] = sortedFiles.map(f => ({ file: f, path: f.name }))
      setFiles(prev => [...prev, ...newEntries])
    },
    [onError]
  )

  useEffect(() => {
    if (disabled) return

    let counter = 0

    function handleDragEnter(e: DragEvent) {
      if (Array.from(e.dataTransfer?.types || []).includes('Files')) {
        counter++
        setDragActive(true)
      }
    }

    function handleDragLeave() {
      counter = Math.max(counter - 1, 0)
      if (counter === 0) setDragActive(false)
    }

    function handleDragOver(e: DragEvent) {
      e.preventDefault()
    }

    async function handleDrop(e: DragEvent) {
      e.preventDefault()
      counter = 0
      setDragActive(false)

      const items = Array.from(e.dataTransfer?.items || [])
      const allFiles: File[] = []

      if (items.length > 0 && typeof items[0].webkitGetAsEntry === 'function') {
        for (const item of items) {
          const entry = item.webkitGetAsEntry()
          if (entry) {
            if (entry.isFile) {
              const file = item.getAsFile()
              if (file) allFiles.push(file)
            } else if (entry.isDirectory) {
              try {
                const extractedFiles = await extractQMDsFromFolder(entry)
                allFiles.push(...extractedFiles)
              } catch (err) {
                console.error('Failed to extract from folder:', err)
              }
            }
          }
        }
      } else {
        const dropFiles = Array.from(e.dataTransfer?.files || [])
        allFiles.push(...dropFiles)
      }

      if (allFiles.length > 0) {
        processFiles(allFiles)
      }
    }

    window.addEventListener('dragenter', handleDragEnter)
    window.addEventListener('dragleave', handleDragLeave)
    window.addEventListener('dragover', handleDragOver)
    window.addEventListener('drop', handleDrop)

    return () => {
      window.removeEventListener('dragenter', handleDragEnter)
      window.removeEventListener('dragleave', handleDragLeave)
      window.removeEventListener('dragover', handleDragOver)
      window.removeEventListener('drop', handleDrop)
    }
  }, [disabled, processFiles])

  const handleClick = () => {
    if (!disabled && fileInputRef.current) {
      fileInputRef.current.click()
    }
  }

  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputFiles = Array.from(e.target.files || [])
    if (inputFiles.length > 0) {
      processFiles(inputFiles)
    }
    e.target.value = ''
  }

  const removeFile = useCallback((index: number) => {
    setFiles(prev => prev.filter((_, i) => i !== index))
  }, [])

  const handleSubmit = useCallback(() => {
    if (files.length === 0) return
    onUpload(
      files.map(f => f.file),
      files.map(f => f.path)
    )
  }, [files, onUpload])

  const clearAll = useCallback(() => {
    setFiles([])
  }, [])

  return (
    <div className="space-y-4">
      <div
        onClick={handleClick}
        className={cn(
          'border-2 border-dashed rounded-lg p-8 text-center transition-colors cursor-pointer',
          dragActive
            ? 'border-primary bg-primary/5'
            : 'border-muted-foreground/25 hover:border-muted-foreground/50',
          disabled && 'opacity-50 pointer-events-none cursor-not-allowed'
        )}
      >
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept={Object.values(ACCEPT_CONFIG).flat().join(',')}
          onChange={handleFileInputChange}
          className="hidden"
          disabled={disabled}
        />
        <Upload className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
        <p className="text-sm text-muted-foreground mb-2">
          Drag and drop .qmd files, folders, or ZIP archives anywhere
        </p>
        <span className="text-primary text-sm">
          or click to browse
        </span>
      </div>

      {files.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">
              {files.length} file{files.length !== 1 ? 's' : ''} selected
            </span>
            <button
              onClick={clearAll}
              className="text-xs text-muted-foreground hover:text-foreground"
            >
              Clear all
            </button>
          </div>
          <div className="max-h-48 overflow-y-auto space-y-1">
            {files.map((entry, index) => (
              <div
                key={index}
                className="flex items-center justify-between py-1 px-2 bg-muted/50 rounded text-sm"
              >
                <div className="flex items-center gap-2 min-w-0">
                  <File className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span className="truncate">{entry.path}</span>
                </div>
                <button
                  onClick={() => removeFile(index)}
                  className="shrink-0 p-1 hover:bg-muted rounded"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            ))}
          </div>
          <Button onClick={handleSubmit} className="w-full" disabled={disabled}>
            Hash {files.length} file{files.length !== 1 ? 's' : ''}
          </Button>
        </div>
      )}
    </div>
  )
}

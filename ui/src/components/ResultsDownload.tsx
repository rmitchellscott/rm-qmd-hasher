import { Download, CheckCircle, XCircle, RotateCcw } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface FileResult {
  name: string;
  path: string;
  status: string;
  error?: string;
}

interface ResultsDownloadProps {
  jobId: string;
  files: FileResult[];
  onReset: () => void;
}

export function ResultsDownload({ jobId, files, onReset }: ResultsDownloadProps) {
  const successFiles = files.filter((f) => f.status === 'success');
  const failedFiles = files.filter((f) => f.status === 'error');

  const handleDownload = () => {
    window.location.href = `/api/download/${jobId}`;
  };

  return (
    <div className="space-y-4">
      <div className="text-center py-4">
        <CheckCircle className="mx-auto h-12 w-12 text-green-500 mb-2" />
        <h3 className="font-semibold text-lg">Hashing Complete</h3>
        <p className="text-sm text-muted-foreground">
          {successFiles.length} of {files.length} file
          {files.length !== 1 ? 's' : ''} hashed successfully
        </p>
      </div>

      {files.length > 0 && (
        <div className="max-h-48 overflow-y-auto space-y-1 border rounded-lg p-2">
          {files.map((file, index) => (
            <div
              key={index}
              className="flex items-center gap-2 py-1 px-2 text-sm"
            >
              {file.status === 'success' ? (
                <CheckCircle className="h-4 w-4 text-green-500 shrink-0" />
              ) : (
                <XCircle className="h-4 w-4 text-destructive shrink-0" />
              )}
              <span className="truncate flex-1">{file.path}</span>
              {file.error && (
                <span className="text-xs text-destructive truncate max-w-[150px]">
                  {file.error}
                </span>
              )}
            </div>
          ))}
        </div>
      )}

      {failedFiles.length > 0 && (
        <p className="text-sm text-destructive text-center">
          {failedFiles.length} file{failedFiles.length !== 1 ? 's' : ''} failed to hash
        </p>
      )}

      <div className="flex gap-2">
        <Button onClick={onReset} variant="outline" className="flex-1">
          <RotateCcw className="h-4 w-4 mr-2" />
          Hash More Files
        </Button>
        {successFiles.length > 0 && (
          <Button onClick={handleDownload} className="flex-1">
            <Download className="h-4 w-4 mr-2" />
            Download {successFiles.length > 1 ? 'All' : 'File'}
          </Button>
        )}
      </div>
    </div>
  );
}

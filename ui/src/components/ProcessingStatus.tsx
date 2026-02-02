import { Loader2 } from 'lucide-react';
import { Progress } from '@/components/ui/progress';

interface JobStatus {
  status: string;
  message: string;
  progress: number;
  operation?: string;
  fileCount?: number;
}

interface ProcessingStatusProps {
  status: JobStatus;
}

export function ProcessingStatus({ status }: ProcessingStatusProps) {
  return (
    <div className="space-y-4 py-4">
      <div className="flex items-center justify-center gap-2">
        <Loader2 className="h-5 w-5 animate-spin text-primary" />
        <span className="font-medium">{status.message || 'Processing...'}</span>
      </div>

      <Progress value={status.progress} className="h-2" />

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>{status.operation || status.status}</span>
        <span>{status.progress}%</span>
      </div>

      {status.fileCount !== undefined && status.fileCount > 0 && (
        <p className="text-center text-sm text-muted-foreground">
          Processing {status.fileCount} file{status.fileCount !== 1 ? 's' : ''}...
        </p>
      )}
    </div>
  );
}

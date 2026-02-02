import { useState, useEffect, useCallback } from 'react';
import { ThemeProvider } from 'next-themes';
import { Toaster } from '@/components/ui/sonner';
import { toast } from 'sonner';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { VersionSelector } from '@/components/VersionSelector';
import { FileDropzone } from '@/components/FileDropzone';
import { ProcessingStatus } from '@/components/ProcessingStatus';
import { ResultsDownload } from '@/components/ResultsDownload';
import ThemeSwitcher from '@/components/ThemeSwitcher';

interface OSVersionInfo {
  version: string;
  devices: string[];
  deviceCount: number;
}

interface AppVersionInfo {
  version: string;
  commit: string;
  buildTime: string;
}

interface FileResult {
  name: string;
  path: string;
  status: string;
  error?: string;
}

interface JobStatus {
  status: string;
  message: string;
  progress: number;
  operation?: string;
  files?: FileResult[];
  fileCount?: number;
}

type AppState = 'idle' | 'uploading' | 'processing' | 'complete' | 'error';

function App() {
  const [versions, setVersions] = useState<OSVersionInfo[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<string>('');
  const [appState, setAppState] = useState<AppState>('idle');
  const [jobId, setJobId] = useState<string | null>(null);
  const [jobStatus, setJobStatus] = useState<JobStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [appVersion, setAppVersion] = useState<AppVersionInfo | null>(null);

  useEffect(() => {
    fetch('/api/versions')
      .then(res => res.json())
      .then(data => {
        setVersions(data.versions || []);
        if (data.versions?.length > 0) {
          setSelectedVersion(data.versions[0].version);
        }
      })
      .catch(err => {
        console.error('Failed to fetch versions:', err);
        toast.error('Failed to load versions');
      });

    fetch('/api/version')
      .then(res => res.json())
      .then(data => setAppVersion(data))
      .catch(err => console.error('Failed to fetch app version:', err));
  }, []);

  const handleUpload = useCallback(async (files: File[], paths: string[]) => {
    if (!selectedVersion) {
      toast.error('Please select a version');
      return;
    }

    setAppState('uploading');
    setError(null);

    const formData = new FormData();
    formData.append('version', selectedVersion);

    files.forEach((file, index) => {
      formData.append('files', file);
      formData.append('paths', paths[index]);
    });

    try {
      const response = await fetch('/api/hash', {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || 'Upload failed');
      }

      const data = await response.json();
      setJobId(data.jobId);
      setAppState('processing');

      subscribeToJob(data.jobId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed');
      setAppState('error');
      toast.error(err instanceof Error ? err.message : 'Upload failed');
    }
  }, [selectedVersion]);

  const subscribeToJob = useCallback((id: string) => {
    const ws = new WebSocket(`${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/status/ws/${id}`);

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data) as JobStatus;
      setJobStatus(data);

      if (data.status === 'success') {
        setAppState('complete');
        toast.success('Hashing complete!');
        ws.close();
      } else if (data.status === 'error') {
        setAppState('error');
        setError(data.message);
        toast.error(data.message);
        ws.close();
      }
    };

    ws.onerror = () => {
      pollJob(id);
      ws.close();
    };
  }, []);

  const pollJob = useCallback(async (id: string) => {
    const checkStatus = async () => {
      try {
        const response = await fetch(`/api/results/${id}`);
        const data = await response.json();
        setJobStatus(data);

        if (data.status === 'success') {
          setAppState('complete');
          toast.success('Hashing complete!');
          return;
        } else if (data.status === 'error') {
          setAppState('error');
          setError(data.message);
          toast.error(data.message);
          return;
        }

        setTimeout(checkStatus, 1000);
      } catch (err) {
        console.error('Polling error:', err);
        setTimeout(checkStatus, 2000);
      }
    };

    checkStatus();
  }, []);

  const handleReset = useCallback(() => {
    setAppState('idle');
    setJobId(null);
    setJobStatus(null);
    setError(null);
  }, []);

  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
      <div className="min-h-screen flex flex-col bg-background">
        <header className="flex items-center justify-between px-8 py-2">
          <h1 className="text-2xl font-bold">reMarkable QMD Hasher</h1>
          <ThemeSwitcher />
        </header>

        <main className="flex-1 px-8 pb-8">
          <div className="max-w-2xl mx-auto">
            <Card>
              <CardHeader>
                <CardTitle>Hash QMD Files</CardTitle>
                <CardDescription>
                  Upload .qmd files and hash them using the GCD hashtab for your selected OS version.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <VersionSelector
                  versions={versions}
                  selectedVersion={selectedVersion}
                  onVersionChange={setSelectedVersion}
                  disabled={appState !== 'idle'}
                />

                {appState === 'idle' && (
                  <FileDropzone onUpload={handleUpload} disabled={!selectedVersion} />
                )}

                {(appState === 'uploading' || appState === 'processing') && jobStatus && (
                  <ProcessingStatus status={jobStatus} />
                )}

                {appState === 'complete' && jobId && jobStatus && (
                  <ResultsDownload
                    jobId={jobId}
                    files={jobStatus.files || []}
                    onReset={handleReset}
                  />
                )}

                {appState === 'error' && (
                  <div className="text-center py-8">
                    <p className="text-destructive mb-4">{error}</p>
                    <button
                      onClick={handleReset}
                      className="text-primary underline hover:no-underline"
                    >
                      Try again
                    </button>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </main>

        {appVersion && (
          <footer className="py-2">
            <div className="text-center text-sm text-muted-foreground">
              <span>{appVersion.version} • </span>
              <a
                href="https://github.com/rmitchellscott/rm-qmd-hasher"
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-foreground hover:underline"
              >
                GitHub
              </a>
            </div>
          </footer>
        )}
      </div>
      <Toaster />
    </ThemeProvider>
  );
}

export default App;

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Label } from '@/components/ui/label';

interface VersionInfo {
  version: string;
  devices: string[];
  deviceCount: number;
}

interface VersionSelectorProps {
  versions: VersionInfo[];
  selectedVersion: string;
  onVersionChange: (version: string) => void;
  disabled?: boolean;
}

export function VersionSelector({
  versions,
  selectedVersion,
  onVersionChange,
  disabled,
}: VersionSelectorProps) {
  return (
    <div className="space-y-2">
      <Label htmlFor="version-select">Target OS Version</Label>
      <Select
        value={selectedVersion}
        onValueChange={onVersionChange}
        disabled={disabled}
      >
        <SelectTrigger id="version-select" className="w-full">
          <SelectValue placeholder="Select a version" />
        </SelectTrigger>
        <SelectContent>
          {versions.map((v) => (
            <SelectItem key={v.version} value={v.version}>
              {v.version} ({v.deviceCount} device{v.deviceCount !== 1 ? 's' : ''})
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {selectedVersion && (
        <p className="text-xs text-muted-foreground">
          Files will be hashed with the GCD hashtab for this version.
        </p>
      )}
    </div>
  );
}

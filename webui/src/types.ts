export interface TorrentStatus {
  infoHash: string;
  name: string;
  uploaded: number;
  seeders: number;
  leechers: number;
  active: boolean;
  addedAt: string;
  lastError?: string;
  ratio: number;
}

export interface ExcludedTorrent {
  hash: string;
  name: string;
  reason: string;
}

export interface QbStatus {
  enabled: boolean;
  lastPollAt?: string;
  lastError?: string;
  torrentsTracked?: number;
  excluded?: ExcludedTorrent[];
}

export interface RssFeed {
  name: string;
  url: string;
  pollInterval: string;
}

export interface QbExclude {
  trackers: string[];
  tags: string[];
  maxSeeders: number;
  minLeechers: number;
  evict: boolean;
}

export interface QbConfig {
  enabled: boolean;
  url: string;
  username: string;
  password: string;
  passwordSet: boolean;
  pollInterval: string;
  category: string;
  exclude?: QbExclude;
}

export interface Config {
  listenAddr: string;
  minUploadRateKBs: number;
  maxUploadRateKBs: number;
  simultaneousSeed: number;
  clientProfile: string;
  keepTorrentWithZeroLeechers: boolean;
  uploadRatioTarget: number;
  torrentsDir: string;
  archiveDir: string;
  profilesDir: string;
  statePath: string;
  rss: RssFeed[];
  qbittorrent: QbConfig | null;
}

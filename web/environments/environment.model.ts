export type EnvironmentName = 'local' | 'dev' | 'qa' | 'vps' | 'prod';

export interface Environment {
  readonly production: boolean;
  readonly name: EnvironmentName;
  readonly apiUrl: string;
}

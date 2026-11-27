import { Injectable, signal } from '@angular/core';

/** Sections of the command list. Search hits use their own sections (Proyectos, Incidencias). */
export type CommandGroup = 'Acciones' | 'Navegación';

export interface Command {
  readonly id: string;
  /** Spanish label shown in the palette. */
  readonly title: string;
  /** Extra words that match the query (synonyms, English names). */
  readonly keywords?: readonly string[];
  readonly group: CommandGroup;
  readonly run: () => void;
}

/**
 * Registry of palette commands. Features register their commands and call the returned function
 * to remove them again. Insertion order is the display order, and registering an id that already
 * exists replaces the command in place.
 */
@Injectable({ providedIn: 'root' })
export class CommandRegistry {
  private readonly store = signal<readonly Command[]>([]);

  /** Every registered command, in registration order. */
  readonly commands = this.store.asReadonly();

  /**
   * Adds a command. The returned function removes it, but only while the stored command is still
   * this one, so a late cleanup cannot remove a newer registration with the same id.
   */
  register(command: Command): () => void {
    this.store.update((current) => {
      const index = current.findIndex((item) => item.id === command.id);
      if (index === -1) {
        return [...current, command];
      }
      return current.map((item, position) => (position === index ? command : item));
    });
    return () => {
      this.store.update((current) => {
        if (!current.includes(command)) {
          return current;
        }
        return current.filter((item) => item !== command);
      });
    };
  }

  /** Test helper: empties the registry. */
  clear(): void {
    this.store.set([]);
  }
}

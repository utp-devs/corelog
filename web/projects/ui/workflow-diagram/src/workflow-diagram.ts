import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import {
  ANY_NODE_ID,
  BreakpointService,
  LayoutEdge,
  LayoutNode,
  WorkflowStatusRef,
  WorkflowTransition,
  layoutWorkflow,
  workflowCategoryVisual,
} from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';

const MIN_SCALE = 0.4;
const MAX_SCALE = 3;
const ZOOM_STEP = 1.2;
/** Pixels of label text per character, a rough width used to size the label plates. */
const LABEL_CHAR = 7.4;
const LABEL_PADDING = 12;

interface DiagramEdge extends LayoutEdge {
  readonly plateWidth: number;
  readonly transitionId: string;
  readonly selected: boolean;
}

interface DiagramNode extends LayoutNode {
  readonly category: 'todo' | 'progress' | 'done' | 'any';
  readonly initial: boolean;
}

/**
 * Auto-laid-out diagram of a workflow: one column per step away from the initial status, curved edges with
 * arrowheads and labels. Pan with the pointer, zoom with the wheel or the buttons, fit to view. The nodes can
 * be focused and chosen with the keyboard. The table of transitions is the text alternative of the diagram.
 */
@Component({
  selector: 'app-workflow-diagram',
  imports: [AppTooltip, Button],
  templateUrl: './workflow-diagram.html',
  styleUrl: './workflow-diagram.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowDiagram {
  private readonly breakpoints = inject(BreakpointService);
  /** Nodes are focusable controls only where they are big enough to hit; on phones the table is the way in. */
  protected readonly interactive = this.breakpoints.matches('(min-width: 768px)');
  private readonly svg = viewChild<ElementRef<SVGSVGElement>>('svg');

  readonly statuses = input.required<readonly WorkflowStatusRef[]>();
  readonly transitions = input.required<readonly WorkflowTransition[]>();
  readonly initialId = input<string>('');
  readonly selectedTransitionId = input<string | null>(null);

  readonly transitionSelected = output<string>();
  readonly statusSelected = output<string>();

  protected readonly scale = signal(1);
  protected readonly translateX = signal(0);
  protected readonly translateY = signal(0);
  protected readonly panning = signal(false);
  private lastPointer: { x: number; y: number } | null = null;

  protected readonly layout = computed(() => {
    const edges = this.transitions().map((transition) => ({
      id: transition.id,
      from: transition.from,
      to: transition.to,
      label: transition.name,
    }));
    return layoutWorkflow(
      this.statuses().map((status) => ({ id: status.id, label: status.name })),
      edges,
      this.initialId(),
    );
  });

  protected readonly nodes = computed<DiagramNode[]>(() => {
    const byId = new Map(this.statuses().map((status) => [status.id, status] as const));
    return this.layout().nodes.map((node) => {
      const status = byId.get(node.id);
      return {
        ...node,
        category: status ? workflowCategoryVisual(status.category) : 'any',
        initial: node.id === this.initialId(),
      };
    });
  });

  protected readonly edges = computed<DiagramEdge[]>(() =>
    this.layout().edges.map((edge) => {
      const transitionId = edge.id.split(':')[0];
      return {
        ...edge,
        plateWidth: edge.label.length * LABEL_CHAR + LABEL_PADDING,
        transitionId,
        selected: transitionId === this.selectedTransitionId(),
      };
    }),
  );

  protected readonly viewBox = computed(() => `0 0 ${this.layout().width} ${this.layout().height}`);
  protected readonly ratio = computed(() => this.layout().width / this.layout().height);
  protected readonly transform = computed(
    () => `translate(${this.translateX()} ${this.translateY()}) scale(${this.scale()})`,
  );
  protected readonly zoomLabel = computed(() => `${Math.round(this.scale() * 100)} %`);
  protected readonly anyId = ANY_NODE_ID;

  protected zoomIn(): void {
    this.zoomBy(ZOOM_STEP);
  }

  protected zoomOut(): void {
    this.zoomBy(1 / ZOOM_STEP);
  }

  protected fit(): void {
    this.scale.set(1);
    this.translateX.set(0);
    this.translateY.set(0);
  }

  protected onWheel(event: WheelEvent): void {
    event.preventDefault();
    this.zoomBy(event.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP, this.toSvgPoint(event));
  }

  protected onPointerDown(event: PointerEvent): void {
    if (
      event.button !== 0 ||
      (event.target as Element).closest('.workflow-diagram__node, .workflow-diagram__edge')
    ) {
      return;
    }
    this.panning.set(true);
    this.lastPointer = { x: event.clientX, y: event.clientY };
    (event.currentTarget as Element).setPointerCapture?.(event.pointerId);
  }

  protected onPointerMove(event: PointerEvent): void {
    if (!this.panning() || !this.lastPointer) return;
    const ratio = this.pixelRatio();
    this.translateX.update((value) => value + (event.clientX - this.lastPointer!.x) * ratio);
    this.translateY.update((value) => value + (event.clientY - this.lastPointer!.y) * ratio);
    this.lastPointer = { x: event.clientX, y: event.clientY };
  }

  protected onPointerUp(): void {
    this.panning.set(false);
    this.lastPointer = null;
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === '+' || event.key === '=') this.zoomIn();
    else if (event.key === '-') this.zoomOut();
    else if (event.key === '0') this.fit();
    else return;
    event.preventDefault();
  }

  protected onNodeKeydown(event: KeyboardEvent, node: DiagramNode): void {
    if ((event.key === 'Enter' || event.key === ' ') && !node.virtual) {
      event.preventDefault();
      this.statusSelected.emit(node.id);
    }
  }

  protected nodeLabel(node: DiagramNode): string {
    if (node.virtual) return 'Cualquier estado';
    return node.initial ? `${node.label}, estado inicial` : node.label;
  }

  private zoomBy(factor: number, around?: { x: number; y: number }): void {
    const current = this.scale();
    const next = Math.min(MAX_SCALE, Math.max(MIN_SCALE, current * factor));
    if (next === current) return;
    const point = around ?? {
      x: this.layout().width / 2,
      y: this.layout().height / 2,
    };
    // Keep the point under the pointer where it is: the content point stays fixed while the scale changes.
    const contentX = (point.x - this.translateX()) / current;
    const contentY = (point.y - this.translateY()) / current;
    this.translateX.set(point.x - contentX * next);
    this.translateY.set(point.y - contentY * next);
    this.scale.set(next);
  }

  /** Pointer position in the coordinates of the diagram (the viewBox). */
  private toSvgPoint(event: MouseEvent): { x: number; y: number } {
    const svg = this.svg()?.nativeElement;
    const rect = svg?.getBoundingClientRect();
    if (!rect || rect.width === 0 || rect.height === 0) {
      return { x: this.layout().width / 2, y: this.layout().height / 2 };
    }
    const ratio = this.pixelRatio();
    // The viewBox is fitted with "meet": the same ratio applies to both axes and the content is centered.
    const usedWidth = this.layout().width / ratio;
    const usedHeight = this.layout().height / ratio;
    const offsetX = (rect.width - usedWidth) / 2;
    const offsetY = (rect.height - usedHeight) / 2;
    return {
      x: (event.clientX - rect.left - offsetX) * ratio,
      y: (event.clientY - rect.top - offsetY) * ratio,
    };
  }

  /** Diagram units per screen pixel. */
  private pixelRatio(): number {
    const rect = this.svg()?.nativeElement.getBoundingClientRect();
    if (!rect || rect.width === 0 || rect.height === 0) return 1;
    return Math.max(this.layout().width / rect.width, this.layout().height / rect.height);
  }
}

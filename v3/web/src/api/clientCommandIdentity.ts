export class ClientCommandIdentity {
  private snapshot: string | null = null;
  private id: string | null = null;

  forPayload(payload: unknown): string {
    const snapshot = JSON.stringify(payload);
    if (snapshot !== this.snapshot || !this.id) {
      this.snapshot = snapshot;
      this.id = crypto.randomUUID();
    }
    return this.id;
  }

  reset(): void {
    this.snapshot = null;
    this.id = null;
  }
}

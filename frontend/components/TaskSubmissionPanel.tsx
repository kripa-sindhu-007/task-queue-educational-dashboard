"use client";

import { useState } from "react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { submitTask, flushData } from "@/lib/api";
import { Send, Shuffle, Trash2, AlertTriangle } from "lucide-react";

const BATCH_SIZES = [1_000, 5_000, 10_000] as const;
const CONCURRENCY = 50;

export default function TaskSubmissionPanel() {
  const [id, setId] = useState("");
  const [priority, setPriority] = useState(5);
  const [delay, setDelay] = useState(0);
  const [maxRetries, setMaxRetries] = useState(3);
  const [submitting, setSubmitting] = useState(false);
  const [batchProgress, setBatchProgress] = useState<string | null>(null);
  const [toast, setToast] = useState<{
    msg: string;
    type: "success" | "error";
  } | null>(null);

  // Flush confirmation dialog
  const [flushDialogOpen, setFlushDialogOpen] = useState(false);

  // Batch config dialog
  const [batchDialogOpen, setBatchDialogOpen] = useState(false);
  const [batchCount, setBatchCount] = useState(1000);
  const [batchPriorityMin, setBatchPriorityMin] = useState(1);
  const [batchPriorityMax, setBatchPriorityMax] = useState(10);
  const [batchDelayChance, setBatchDelayChance] = useState(30);
  const [batchDelayMax, setBatchDelayMax] = useState(10);
  const [batchMaxRetries, setBatchMaxRetries] = useState(3);

  const showToast = (msg: string, type: "success" | "error") => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const taskId = id.trim() || `task-${Date.now()}`;
    setSubmitting(true);
    try {
      await submitTask({ id: taskId, priority, delay, max_retries: maxRetries });
      showToast(`Task "${taskId}" submitted`, "success");
      setId("");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Submit failed", "error");
    } finally {
      setSubmitting(false);
    }
  };

  const openBatchDialog = (count: number) => {
    setBatchCount(count);
    setBatchDialogOpen(true);
  };

  const executeBatch = async () => {
    setBatchDialogOpen(false);
    setSubmitting(true);
    setBatchProgress(`0 / ${batchCount.toLocaleString()}`);
    let sent = 0;
    try {
      const ts = Date.now();
      for (let i = 0; i < batchCount; i += CONCURRENCY) {
        const chunk = Math.min(CONCURRENCY, batchCount - i);
        const promises = Array.from({ length: chunk }, (_, j) => {
          const pRange = batchPriorityMax - batchPriorityMin + 1;
          const p = Math.floor(Math.random() * pRange) + batchPriorityMin;
          const hasDelay = Math.random() * 100 < batchDelayChance;
          const d = hasDelay ? Math.floor(Math.random() * batchDelayMax) + 1 : 0;
          return submitTask({
            id: `batch-${ts}-${i + j}`,
            priority: p,
            delay: d,
            max_retries: batchMaxRetries,
          });
        });
        await Promise.all(promises);
        sent += chunk;
        setBatchProgress(`${sent.toLocaleString()} / ${batchCount.toLocaleString()}`);
      }
      showToast(`${batchCount.toLocaleString()} tasks submitted`, "success");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Batch submit failed", "error");
    } finally {
      setSubmitting(false);
      setBatchProgress(null);
    }
  };

  const handleFlush = async () => {
    setFlushDialogOpen(false);
    setSubmitting(true);
    try {
      await flushData();
      showToast("All data cleared", "success");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Flush failed", "error");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Submit Task</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-xs text-muted-foreground -mt-1">
            Submit a task to see it flow through the pipeline: submit → queue → worker → outcome.
          </p>
          <form onSubmit={handleSubmit} className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="task-id">Task ID (auto-generated if empty)</Label>
              <Input
                id="task-id"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="my-task-1"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="priority">Priority (1-10, higher = first)</Label>
              <Input
                id="priority"
                type="number"
                min={1}
                max={10}
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="delay">Delay (seconds)</Label>
              <Input
                id="delay"
                type="number"
                min={0}
                value={delay}
                onChange={(e) => setDelay(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="retries">Max Retries</Label>
              <Input
                id="retries"
                type="number"
                min={0}
                max={10}
                value={maxRetries}
                onChange={(e) => setMaxRetries(Number(e.target.value))}
              />
            </div>
            <Button className="w-full cursor-pointer" disabled={submitting} type="submit">
              <Send className="w-4 h-4" />
              {submitting && !batchProgress ? "Submitting…" : "Submit Task"}
            </Button>
          </form>

          {/* Batch submit buttons */}
          <div className="space-y-1.5">
            <Label className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
              Batch Submit
            </Label>
            <div className="grid grid-cols-3 gap-2">
              {BATCH_SIZES.map((size) => (
                <Button
                  key={size}
                  variant="secondary"
                  size="sm"
                  className="cursor-pointer tabular-nums"
                  disabled={submitting}
                  onClick={() => openBatchDialog(size)}
                  type="button"
                >
                  <Shuffle className="w-3 h-3" />
                  {size >= 1000 ? `${size / 1000}k` : size}
                </Button>
              ))}
            </div>
            {batchProgress && (
              <p className="text-xs text-muted-foreground text-center tabular-nums">
                Sending: {batchProgress}
              </p>
            )}
          </div>

          {/* Flush data */}
          <div className="border-t border-border pt-3">
            <Button
              className="w-full cursor-pointer"
              variant="destructive"
              size="sm"
              disabled={submitting}
              onClick={() => setFlushDialogOpen(true)}
              type="button"
            >
              <Trash2 className="w-3.5 h-3.5" />
              Clear All Data
            </Button>
            <p className="text-[10px] text-muted-foreground mt-1 text-center">
              Deletes all queues, metrics, events &amp; dead-letter data from Redis
            </p>
          </div>

          {toast && (
            <div
              className={`toast rounded-lg border px-4 py-2.5 text-sm font-semibold shadow-lg ${
                toast.type === "success"
                  ? "border-state-succeeded/25 bg-state-succeeded/15 text-state-succeeded"
                  : "border-state-failed/25 bg-state-failed/15 text-state-failed"
              }`}
            >
              {toast.msg}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Flush Confirmation Dialog */}
      <Dialog open={flushDialogOpen} onOpenChange={setFlushDialogOpen}>
        <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="w-5 h-5" />
            Clear All Data?
          </DialogTitle>
          <DialogDescription>
            This will permanently delete all queues, metrics, events, worker states,
            and dead-letter data from Redis. This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" size="sm" className="cursor-pointer" onClick={() => setFlushDialogOpen(false)}>
            Cancel
          </Button>
          <Button variant="destructive" size="sm" className="cursor-pointer" onClick={handleFlush}>
            <Trash2 className="w-3.5 h-3.5" />
            Yes, Clear Everything
          </Button>
        </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Batch Config Dialog */}
      <Dialog open={batchDialogOpen} onOpenChange={setBatchDialogOpen}>
        <DialogContent>
        <DialogHeader>
          <DialogTitle>Configure Batch — {batchCount.toLocaleString()} Tasks</DialogTitle>
          <DialogDescription>
            Customize the random values for the batch of tasks.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Priority Min</Label>
              <Input
                type="number"
                min={1}
                max={10}
                value={batchPriorityMin}
                onChange={(e) => setBatchPriorityMin(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1">
              <Label>Priority Max</Label>
              <Input
                type="number"
                min={1}
                max={10}
                value={batchPriorityMax}
                onChange={(e) => setBatchPriorityMax(Number(e.target.value))}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Delay Chance (%)</Label>
              <Input
                type="number"
                min={0}
                max={100}
                value={batchDelayChance}
                onChange={(e) => setBatchDelayChance(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1">
              <Label>Max Delay (sec)</Label>
              <Input
                type="number"
                min={1}
                max={120}
                value={batchDelayMax}
                onChange={(e) => setBatchDelayMax(Number(e.target.value))}
              />
            </div>
          </div>
          <div className="space-y-1">
            <Label>Max Retries</Label>
            <Input
              type="number"
              min={0}
              max={10}
              value={batchMaxRetries}
              onChange={(e) => setBatchMaxRetries(Number(e.target.value))}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" className="cursor-pointer" onClick={() => setBatchDialogOpen(false)}>
            Cancel
          </Button>
          <Button size="sm" className="cursor-pointer tabular-nums" onClick={executeBatch}>
            <Shuffle className="w-3.5 h-3.5" />
            Send {batchCount.toLocaleString()} Tasks
          </Button>
        </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

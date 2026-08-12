import Link from "next/link";
import { notFound } from "next/navigation";
import {
  ArrowLeft,
  ArrowRight,
  Clock,
  CheckCircle,
  ExternalLink,
  Sparkles,
  Lock,
} from "lucide-react";
import {
  chapters,
  getChapter,
  getChapterNav,
  phaseTone,
  type Chapter,
} from "@/lib/learn";
import MarkdownContent from "@/components/learn/MarkdownContent";
import ReadingProgressBar from "@/components/learn/ReadingProgressBar";
import ChapterReadMarker from "@/components/learn/ChapterReadMarker";
import RevisionQuestions from "@/components/learn/RevisionQuestions";
import InterviewSummary from "@/components/learn/InterviewSummary";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

export function generateStaticParams() {
  return chapters.map((c) => ({ slug: c.slug }));
}

function BackLink() {
  return (
    <Link
      href="/learn"
      className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      Back to Learn
    </Link>
  );
}

function PhaseBadge({ chapter }: { chapter: Chapter }) {
  const tone = phaseTone[chapter.phaseGroup];
  return (
    <Badge
      className={cn(
        "rounded-md font-mono uppercase tracking-[0.1em] ring-1 ring-inset",
        tone.chip,
        tone.ink,
        tone.ring
      )}
    >
      {chapter.phaseGroup}
    </Badge>
  );
}

function ComingSoonState({ chapter }: { chapter: Chapter }) {
  return (
    <div className="mx-auto max-w-[72ch] px-4 py-10 space-y-6">
      <BackLink />
      <Card className="items-center p-8 text-center">
        <span className="flex h-14 w-14 items-center justify-center rounded-xl border border-border bg-muted text-muted-foreground">
          <Lock className="h-7 w-7" aria-hidden="true" />
        </span>
        <PhaseBadge chapter={chapter} />
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            {chapter.title}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {chapter.subtitle}
          </p>
        </div>
        <p className="mx-auto max-w-md text-base leading-7 text-foreground/80">
          This chapter is coming soon. In the meantime, start with the chapters
          that are ready and come back as the curriculum grows.
        </p>
        <Button asChild>
          <Link href="/learn">
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Browse chapters
          </Link>
        </Button>
      </Card>
    </div>
  );
}

function KeyTakeaways({ items }: { items: string[] }) {
  return (
    <section aria-labelledby="takeaways-heading" className="mt-10">
      <h2
        id="takeaways-heading"
        className="mb-3 flex items-center gap-2 text-2xl font-bold tracking-tight text-foreground"
      >
        <Sparkles className="h-6 w-6 text-primary" aria-hidden="true" />
        Key Takeaways
      </h2>
      <Card className="border-l-4 border-l-primary py-5">
        <CardContent>
          <ul className="space-y-3">
            {items.map((item, i) => (
              <li key={i} className="flex items-start gap-2.5">
                <CheckCircle
                  className="mt-0.5 h-5 w-5 shrink-0 text-primary"
                  aria-hidden="true"
                />
                <span className="text-base leading-7 text-foreground/85">
                  {item}
                </span>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </section>
  );
}

function ChapterNavFooter({ slug }: { slug: string }) {
  const { prev, next } = getChapterNav(slug);

  const renderSide = (chapter: Chapter | undefined, dir: "prev" | "next") => {
    if (!chapter) return <div className="flex-1" />;
    const available = chapter.status === "available";
    const label = dir === "prev" ? "Previous" : "Next";
    const content = (
      <Card
        className={cn(
          "h-full gap-1 py-4 transition-colors",
          dir === "next" && "text-right",
          available
            ? "group-hover:border-primary/50 group-focus-visible:border-primary/50"
            : "opacity-60"
        )}
      >
        <CardContent>
          <span
            className={cn(
              "flex items-center gap-1.5 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground",
              dir === "next" && "justify-end"
            )}
          >
            {dir === "prev" && (
              <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
            )}
            {label}
            {dir === "next" && (
              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
            )}
          </span>
          <span className="mt-1 block text-sm font-semibold text-foreground">
            {chapter.title}
          </span>
          {!available && (
            <span
              className={cn(
                "mt-1 inline-flex items-center gap-1 text-[11px] font-medium text-muted-foreground",
                dir === "next" && "flex-row-reverse"
              )}
            >
              <Lock className="h-3 w-3" aria-hidden="true" />
              Coming soon
            </span>
          )}
        </CardContent>
      </Card>
    );

    if (!available) {
      return (
        <div aria-disabled="true" className="flex-1">
          {content}
        </div>
      );
    }
    return (
      <Link
        href={`/learn/${chapter.slug}`}
        className="group flex-1 rounded-xl focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        {content}
      </Link>
    );
  };

  if (!prev && !next) return null;

  return (
    <nav
      aria-label="Chapter navigation"
      className="mt-12 flex flex-col gap-3 sm:flex-row"
    >
      {renderSide(prev, "prev")}
      {renderSide(next, "next")}
    </nav>
  );
}

export default async function ChapterReaderPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const chapter = getChapter(slug);

  // Truly unknown slug → hard 404. Known-but-not-ready → friendly state.
  if (!chapter) notFound();
  if (chapter.status !== "available" || !chapter.body) {
    return <ComingSoonState chapter={chapter} />;
  }

  return (
    <>
      <ReadingProgressBar />
      <ChapterReadMarker slug={chapter.slug} />

      <article className="mx-auto max-w-[72ch] px-4 py-8">
        <header>
          <BackLink />
          <div className="mt-4">
            <PhaseBadge chapter={chapter} />
          </div>
          <h1 className="mt-3 text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
            {chapter.title}
          </h1>
          <p className="mt-2 text-base text-muted-foreground">
            {chapter.subtitle}
          </p>
          <p className="mt-3 inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
            <Clock className="h-3.5 w-3.5" aria-hidden="true" />
            {chapter.readingMinutes} min read
          </p>
        </header>

        <Separator className="mt-6" />

        <div className="mt-8">
          <MarkdownContent>{chapter.body}</MarkdownContent>
        </div>

        {chapter.relatedPanel && (
          <div className="mt-8">
            <Button asChild>
              <Link href={chapter.relatedPanel.href}>
                <ExternalLink className="h-4 w-4" aria-hidden="true" />
                See it live: {chapter.relatedPanel.label}
              </Link>
            </Button>
          </div>
        )}

        {chapter.keyTakeaways && chapter.keyTakeaways.length > 0 && (
          <KeyTakeaways items={chapter.keyTakeaways} />
        )}

        {chapter.interviewSummary && (
          <InterviewSummary summary={chapter.interviewSummary} />
        )}

        {chapter.revisionQuestions && chapter.revisionQuestions.length > 0 && (
          <RevisionQuestions questions={chapter.revisionQuestions} />
        )}

        <ChapterNavFooter slug={chapter.slug} />
      </article>
    </>
  );
}

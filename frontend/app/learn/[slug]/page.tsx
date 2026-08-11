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
import { cn } from "@/lib/utils";

export function generateStaticParams() {
  return chapters.map((c) => ({ slug: c.slug }));
}

function BackLink() {
  return (
    <Link
      href="/learn"
      className="inline-flex items-center gap-1.5 text-sm font-semibold text-muted-foreground hover:text-foreground"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      Back to Learn
    </Link>
  );
}

function ComingSoonState({ chapter }: { chapter: Chapter }) {
  const tone = phaseTone[chapter.phaseGroup];
  return (
    <div className="mx-auto max-w-[72ch] px-4 py-10 space-y-6">
      <BackLink />
      <div className="clay rounded-2xl bg-card p-8 text-center">
        <div className="clay-chip mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
          <Lock className="h-7 w-7" aria-hidden="true" />
        </div>
        <span
          className={cn(
            "clay-chip mt-5 inline-block rounded-lg px-3 py-1 font-display text-xs font-bold",
            tone.chip,
            tone.ink
          )}
        >
          {chapter.phaseGroup}
        </span>
        <h1 className="mt-3 font-display text-2xl font-bold tracking-tight text-foreground">
          {chapter.title}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">{chapter.subtitle}</p>
        <p className="mx-auto mt-5 max-w-md text-[15px] leading-7 text-foreground/80">
          This chapter is coming soon. In the meantime, start with the chapters
          that are ready and come back as the curriculum grows.
        </p>
        <Link
          href="/learn"
          className="clay-btn mt-6 inline-flex items-center gap-2 bg-primary px-5 py-2.5 text-sm font-display font-semibold text-primary-foreground"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          Browse chapters
        </Link>
      </div>
    </div>
  );
}

function KeyTakeaways({ items }: { items: string[] }) {
  return (
    <section aria-labelledby="takeaways-heading" className="mt-10">
      <h2
        id="takeaways-heading"
        className="font-display text-2xl font-bold tracking-tight text-foreground mb-3 flex items-center gap-2"
      >
        <Sparkles className="h-6 w-6 text-mint-ink" aria-hidden="true" />
        Key Takeaways
      </h2>
      <ul className="clay-inset space-y-3 rounded-xl bg-mint-soft/50 p-5">
        {items.map((item, i) => (
          <li key={i} className="flex items-start gap-2.5">
            <CheckCircle
              className="mt-0.5 h-5 w-5 shrink-0 text-mint-ink"
              aria-hidden="true"
            />
            <span className="text-[15px] leading-7 text-foreground/85">
              {item}
            </span>
          </li>
        ))}
      </ul>
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
      <>
        <span className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
          {dir === "prev" && <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />}
          {label}
          {dir === "next" && <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />}
        </span>
        <span className="mt-1 block font-display text-sm font-semibold text-foreground">
          {chapter.title}
        </span>
        {!available && (
          <span className="mt-1 inline-flex items-center gap-1 text-[11px] font-semibold text-muted-foreground">
            <Lock className="h-3 w-3" aria-hidden="true" />
            Coming soon
          </span>
        )}
      </>
    );

    const base = cn(
      "flex-1 rounded-2xl bg-card p-4",
      dir === "next" && "text-right"
    );

    if (!available) {
      return (
        <div aria-disabled="true" className={cn(base, "clay opacity-55")}>
          {content}
        </div>
      );
    }
    return (
      <Link href={`/learn/${chapter.slug}`} className={cn(base, "clay-btn")}>
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

  const tone = phaseTone[chapter.phaseGroup];

  return (
    <>
      <ReadingProgressBar />
      <ChapterReadMarker slug={chapter.slug} />

      <article className="mx-auto max-w-[72ch] px-4 py-8">
        <header>
          <BackLink />
          <span
            className={cn(
              "clay-chip mt-4 inline-block rounded-lg px-3 py-1 font-display text-xs font-bold",
              tone.chip,
              tone.ink
            )}
          >
            {chapter.phaseGroup}
          </span>
          <h1 className="mt-3 font-display text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
            {chapter.title}
          </h1>
          <p className="mt-2 text-base text-muted-foreground">
            {chapter.subtitle}
          </p>
          <p className="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
            <Clock className="h-3.5 w-3.5" aria-hidden="true" />
            {chapter.readingMinutes} min read
          </p>
        </header>

        <div className="mt-8">
          <MarkdownContent>{chapter.body}</MarkdownContent>
        </div>

        {chapter.relatedPanel && (
          <Link
            href={chapter.relatedPanel.href}
            className="clay-btn mt-8 inline-flex items-center gap-2 bg-primary px-5 py-2.5 text-sm font-display font-semibold text-primary-foreground"
          >
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
            See it live: {chapter.relatedPanel.label}
          </Link>
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

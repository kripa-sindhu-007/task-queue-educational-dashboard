"use client";

import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

const components: Components = {
  h1: ({ children }) => (
    <h1 className="mt-10 mb-4 text-3xl font-bold tracking-tight text-foreground first:mt-0">
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 className="mt-12 mb-4 text-2xl font-bold tracking-tight text-foreground first:mt-0">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="mt-8 mb-3 text-lg font-semibold tracking-tight text-foreground">
      {children}
    </h3>
  ),
  p: ({ children }) => (
    <p className="my-4 text-base leading-7 text-foreground/85">{children}</p>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      className="font-medium text-primary underline decoration-primary/40 underline-offset-2 transition-colors hover:decoration-primary"
    >
      {children}
    </a>
  ),
  ul: ({ children }) => (
    <ul className="my-4 space-y-2 pl-5 list-disc text-base leading-7 text-foreground/85 marker:text-primary/70">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="my-4 space-y-2 pl-5 list-decimal text-base leading-7 text-foreground/85 marker:font-semibold marker:text-primary/70">
      {children}
    </ol>
  ),
  li: ({ children }) => <li className="pl-1">{children}</li>,
  strong: ({ children }) => (
    <strong className="font-semibold text-foreground">{children}</strong>
  ),
  blockquote: ({ children }) => (
    <blockquote className="my-6 rounded-r-lg border-l-4 border-primary bg-muted/50 px-5 py-3 text-base leading-7 text-foreground/80 [&_p]:my-2 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0">
      {children}
    </blockquote>
  ),
  code: ({ className, children }) => {
    // Fenced code blocks carry a language className; inline code does not.
    const isBlock = /language-/.test(className ?? "");
    if (isBlock) {
      return (
        <code className="whitespace-pre font-mono text-[13px] leading-6 text-foreground/90">
          {children}
        </code>
      );
    }
    return (
      <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-[0.85em] text-foreground">
        {children}
      </code>
    );
  },
  pre: ({ children }) => (
    <pre className="my-6 overflow-x-auto rounded-lg border border-border bg-background p-4">
      {children}
    </pre>
  ),
  table: ({ children }) => (
    <div className="my-6 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        {children}
      </table>
    </div>
  ),
  thead: ({ children }) => (
    <thead className="bg-muted/60">{children}</thead>
  ),
  th: ({ children }) => (
    <th className="border-b border-border px-4 py-2.5 text-[13px] font-semibold text-foreground">
      {children}
    </th>
  ),
  tr: ({ children }) => (
    <tr className="border-b border-border/50 last:border-0 even:bg-muted/20">
      {children}
    </tr>
  ),
  td: ({ children }) => (
    <td className="px-4 py-2.5 align-top text-foreground/85">{children}</td>
  ),
  hr: () => <hr className="my-10 border-border/60" />,
};

export default function MarkdownContent({ children }: { children: string }) {
  return (
    <div className="max-w-[68ch]">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}

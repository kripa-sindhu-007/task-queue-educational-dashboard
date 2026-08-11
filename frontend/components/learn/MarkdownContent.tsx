"use client";

import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

const components: Components = {
  h1: ({ children }) => (
    <h1 className="font-display text-3xl font-bold tracking-tight text-foreground mt-8 mb-4 first:mt-0">
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 className="font-display text-2xl font-bold tracking-tight text-foreground mt-10 mb-3 first:mt-0">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="font-display text-xl font-semibold tracking-tight text-foreground mt-7 mb-2">
      {children}
    </h3>
  ),
  p: ({ children }) => (
    <p className="text-[15px] leading-7 text-foreground/85 my-4">{children}</p>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      className="font-semibold text-primary underline decoration-primary/30 underline-offset-2 hover:decoration-primary"
    >
      {children}
    </a>
  ),
  ul: ({ children }) => (
    <ul className="my-4 space-y-2 pl-5 list-disc marker:text-primary/60 text-[15px] leading-7 text-foreground/85">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="my-4 space-y-2 pl-5 list-decimal marker:text-primary/70 marker:font-semibold text-[15px] leading-7 text-foreground/85">
      {children}
    </ol>
  ),
  li: ({ children }) => <li className="pl-1">{children}</li>,
  strong: ({ children }) => (
    <strong className="font-bold text-foreground">{children}</strong>
  ),
  blockquote: ({ children }) => (
    <blockquote className="clay-inset my-5 px-5 py-3 text-[15px] leading-7 text-foreground/80 border-l-4 border-primary/40">
      {children}
    </blockquote>
  ),
  code: ({ className, children }) => {
    // Fenced code blocks carry a language className; inline code does not.
    const isBlock = /language-/.test(className ?? "");
    if (isBlock) {
      return (
        <code className="font-mono text-[13px] leading-6 text-foreground/90 whitespace-pre">
          {children}
        </code>
      );
    }
    return (
      <code className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-[0.85em] text-grape-ink">
        {children}
      </code>
    );
  },
  pre: ({ children }) => (
    <pre className="clay-inset my-5 overflow-x-auto rounded-xl p-4">
      {children}
    </pre>
  ),
  table: ({ children }) => (
    <div className="clay-inset my-6 overflow-x-auto rounded-xl p-1.5">
      <table className="w-full border-collapse text-left text-[14px]">
        {children}
      </table>
    </div>
  ),
  thead: ({ children }) => <thead>{children}</thead>,
  th: ({ children }) => (
    <th className="border-b border-border/70 px-3 py-2.5 font-display text-[13px] font-bold text-foreground">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="border-b border-border/40 px-3 py-2.5 align-top text-foreground/85">
      {children}
    </td>
  ),
  hr: () => <hr className="my-8 border-border/60" />,
};

export default function MarkdownContent({ children }: { children: string }) {
  return (
    <div className="max-w-[72ch]">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}

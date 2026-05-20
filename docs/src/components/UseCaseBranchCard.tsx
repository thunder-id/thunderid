import React from 'react';
import Link from '@docusaurus/Link';

type UseCaseBranchCardProps = {
  href: string;
  animationClass: string;
  icon: React.ReactNode;
  accentColor: string;
  iconBackground: string;
  category: string;
  title: string;
  description: string;
  bullets: string[];
};

export default function UseCaseBranchCard({
  href,
  animationClass,
  icon,
  accentColor,
  iconBackground,
  category,
  title,
  description,
  bullets,
}: UseCaseBranchCardProps) {
  return (
    <Link
      to={href}
      className={`uc-card uc-branch-card ${animationClass}`}
      style={{
        ['--uc-branch-accent' as string]: accentColor,
        ['--uc-branch-icon-bg' as string]: iconBackground,
      }}
    >
      <div className="uc-branch-icon">{icon}</div>

      <div className="uc-branch-category">{category}</div>

      <div className="uc-branch-title">{title}</div>

      <div className="uc-branch-description">{description}</div>

      <div className="uc-branch-when">
        <div className="uc-branch-when-label">Choose when</div>

        <ul className="uc-branch-when-list">
          {bullets.map((bullet) => (
            <li key={bullet}>{bullet}</li>
          ))}
        </ul>
      </div>

      <span className="uc-branch-link">View pattern -&gt;</span>
    </Link>
  );
}

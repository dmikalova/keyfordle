import { useEffect } from "react";

interface SEOProps {
  title?: string;
  description?: string;
  keywords?: string;
  canonical?: string;
}

/**
 * Custom hook to manage page SEO metadata
 * @param title - Page title (used as is)
 * @param description - Page meta description
 * @param keywords - Additional keywords for the page
 * @param canonical - Canonical URL for the page
 */
export function useSEO({ title, description, keywords, canonical }: SEOProps = {}) {
  useEffect(() => {
    // Update title
    if (title) {
      document.title = title;
    } else {
      document.title = "KeyFordle";
    }

    // Update meta description
    if (description) {
      const metaDescription = document.querySelector('meta[name="description"]');
      if (metaDescription) {
        metaDescription.setAttribute("content", description);
      }
    }

    // Update keywords
    if (keywords) {
      let metaKeywords = document.querySelector('meta[name="keywords"]');
      if (metaKeywords) {
        const baseKeywords =
          "KeyForge, deck stats, player rankings, card analytics, TCG statistics, KeyForge meta, deck tracker";
        metaKeywords.setAttribute("content", `${keywords}, ${baseKeywords}`);
      }
    }

    // Update canonical URL
    if (canonical) {
      let linkCanonical = document.querySelector('link[rel="canonical"]');
      if (linkCanonical) {
        linkCanonical.setAttribute("href", canonical);
      } else {
        linkCanonical = document.createElement("link");
        linkCanonical.setAttribute("rel", "canonical");
        linkCanonical.setAttribute("href", canonical);
        document.head.appendChild(linkCanonical);
      }
    }

    // Update Open Graph tags
    if (title) {
      const ogTitle = document.querySelector('meta[property="og:title"]');
      if (ogTitle) {
        ogTitle.setAttribute("content", title);
      }
    }

    if (description) {
      const ogDescription = document.querySelector('meta[property="og:description"]');
      if (ogDescription) {
        ogDescription.setAttribute("content", description);
      }
    }

    if (canonical) {
      const ogUrl = document.querySelector('meta[property="og:url"]');
      if (ogUrl) {
        ogUrl.setAttribute("content", canonical);
      }
    }

    // Update Twitter Card tags
    if (title) {
      const twitterTitle = document.querySelector('meta[name="twitter:title"]');
      if (twitterTitle) {
        twitterTitle.setAttribute("content", title);
      }
    }

    if (description) {
      const twitterDescription = document.querySelector('meta[name="twitter:description"]');
      if (twitterDescription) {
        twitterDescription.setAttribute("content", description);
      }
    }

    if (canonical) {
      const twitterUrl = document.querySelector('meta[name="twitter:url"]');
      if (twitterUrl) {
        twitterUrl.setAttribute("content", canonical);
      }
    }
  }, [title, description, keywords, canonical]);
}

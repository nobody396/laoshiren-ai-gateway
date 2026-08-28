import flags from '../../config/public-features.json'

// One switch controls the route, public navigation, authenticated navigation,
// SEO manifest, sitemap, and llms.txt. Flip this back to true only after the
// documentation rewrite has passed review.
export const PUBLIC_DOCS_ENABLED = flags.docs === true

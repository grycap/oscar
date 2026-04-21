# Hands-on Labs

<section class="oscar-hero oscar-lab-hero">
  <p class="oscar-lab-kicker">Training track</p>
  <h1 class="oscar-hero__title">Deploy and execute OSCAR services step by step</h1>
  <p class="oscar-hero__body">
    This section is designed as a practical training path for users. Each lab follows a repeatable sequence:
    deploy an OSCAR service (either via the Dashboard or the CLI), review the generated resources, execute the service synchronously or asynchronously, and analyse the generated output data.
  </p>
  <div class="oscar-hero__actions">
    <a class="oscar-pill" href="../usage-dashboard/">Dashboard guide</a>
    <a class="oscar-pill" href="../oscar-cli/">CLI guide</a>
    <a class="oscar-pill" href="../invoking-sync/">Sync invocations</a>
    <a class="oscar-pill" href="../invoking-async/">Async invocations</a>
  </div>
</section>

<div class="oscar-lab-intro-grid">
  <section class="oscar-lab-panel">
    <p class="oscar-media-card__eyebrow">Proposal</p>
    <h2>What are these labs?</h2>
    <ul>
      <li>Short context and learning goals for the selected OSCAR services.</li>
      <li>A deployment flow based on the Dashboard so new users can follow it without preparing an FDL first.</li>
      <li>Two validation tracks: synchronous invocation for immediate feedback and asynchronous execution through MinIO events.</li>
      <li>A final checkpoint with logs, expected outputs, and cleanup steps.</li>
    </ul>
  </section>
  <section class="oscar-lab-panel">
    <p class="oscar-media-card__eyebrow">Audience</p>
    <h2>Optimized for onboarding and workshops</h2>
    <ul>
      <li>Users only need an OSCAR deployment, Dashboard access, and a sample file to upload.</li>
      <li>The guides stay close to the visual workflow already documented in the Dashboard and invocation sections.</li>
      <li>The same structure can be reused later for other domain-specific OSCAR services.</li>
      <li>Each lab links back to the relevant reference pages instead of duplicating every detail.</li>
    </ul>
  </section>
</div>


## Available labs

<div class="oscar-media-grid">
  <section class="oscar-media-card oscar-lab-feature">
    <div class="oscar-media-card__header">
      <p class="oscar-media-card__eyebrow">Lab 01</p>
      <h2>ImageMagick from OSCAR Hub</h2>
      <p class="oscar-media-card__description">
        Deploy the ImageMagick example from OSCAR Hub, run a synchronous smoke test, and then validate the asynchronous
        file-processing workflow by uploading images to the generated input bucket.
      </p>
    </div>    
    <div class="oscar-media-card__actions">
      <a class="oscar-slide-button" href="../hands-on-imagemagick/">Open lab</a>
    </div>
  </section>
</div>

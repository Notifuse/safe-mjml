# Refonte partielle des fondations (arbre, attributs, MSO)

**Date :** 2026-11-11
**Statut :** En cours. Étape 0 confiée à safe-mjml-3b.
**Priorité :** Haute
**Origine :** analyse d'architecture en lecture seule, faite à la demande de la session safe-mjml-32 sur la branche `fix/mjml-4.18-parity`.

## Décisions de Pierre (2026-11-11)

- **Calendrier :** refonte après le correctif background-url de mj-wrapper. Ce correctif est fait : la branche est commitée, avec 240 fixtures MJML 4.18, toutes vertes.
- **Tests :** durcir la comparaison CSS des tests d'intégration **et** ajouter un snapshot octet pour octet de la sortie de safe-mjml. C'est le filet de sécurité de l'étape 0.
- **Répartition :** safe-mjml-32 décide qui fait quoi. L'étape 0 est confiée à safe-mjml-3b, sur la branche `refactor/step0-safety-net` partie de `fix/mjml-4.18-parity`, dans son propre worktree. safe-mjml-32 traite la code review de `fix/mjml-4.18-parity`, puis P5 et P8.
- **Ordre :** refonte maintenant, par étapes (0, puis 1 à 4), chacune sur sa branche, avec les tests verts à chaque étape. `fix/mjml-4.18-parity` reste en local pour l'instant.
- **Étape 4 :** accepté, la sortie peut être entièrement bufferisée avant la fusion des conditionnels Outlook, comme dans MJML. Mesurer l'effet avec les benchmarks.
- **Pas de dette technique (2026-11-11) :** pour les refontes structurelles, changer les signatures et l'API publique est permis, Notifuse sera adapté si besoin. On supprime l'ancien code au lieu de le déprécier.
- **Validation (2026-11-11) :** balises inconnues, composants mal placés et attributs invalides sont des avertissements (`mjml.WithWarningHandler`, `RenderResult.Warnings`), pas des erreurs, comme la validation souple de MJML. La CLI les écrit sur stderr ; `--strict` la fait sortir en code 1.
- **Priorité du hero (2026-11-11) :** les écarts de mj-hero (`mj-wrapper-hero`, `mj-hero-padding-no-height`) passent en dernier, après le reste de la refonte : « c'est un composant dont personne ne se sert ».
- **Règles de test (2026-11-11) :** avant v0.16.0, la matrice d'attributs générée (`TestMJMLMatrix`, dimensions non rondes, attributs vides), les invariants sur tous les documents (`TestRenderInvariants`, `FuzzRender`) et le diff avec la release précédente (`tools/release-diff.sh`), décrits dans `mjml/testdata/AGENTS.md`.
- **CI et fork (2026-11-11) :** la CI GitHub Actions (go test, -race, GOARCH=amd64, `--check` des références) est ajoutée juste avant v1.0.0. Pierre demande lui-même au support GitHub de détacher le dépôt du réseau de forks de preslavrachev/gomjml.
- **API publique (2026-11-11) :** avant v1.0.0, `mjml/components`, `options`, `styles`, `html` et `globals` passent dans `internal/` ; l'API publique sera `mjml.Render` et ses options, plus `parser`. Rien à changer pour Notifuse, qui n'importe que `mjml`.
- **Relectures (2026-11-11) :** avant chaque release qui change le rendu ou l'API, deux relectures indépendantes du même diff, en parallèle ; un changement de tests ou d'outils seuls passe par des mutations rejouées, sans relecture.
- **API (`RenderFromAST` / `NewFromAST`) :** vérifié le 2026-11-11, Notifuse n'appelle que `mjml.Render(...)` sans option (`pkg/notifuse_mjml/template_compilation.go:939`). Les aligner sur `Render` ne change rien pour Notifuse, seulement pour les autres consommateurs de la bibliothèque. Pierre a validé l'alignement : **fait** sur la branche `fix/render-from-ast` (commit 8c4ec5b). Les quatre points d'entrée passent par le même pipeline, l'ordre des classes de colonne est corrigé à la source, et `TestPublicEntryPointsAgree` vérifie qu'ils produisent le même HTML sur les 240 documents de test.

## Verdict

Une refonte partielle ciblée, pas une réécriture. Le rendu des composants feuilles (text, button, image, divider…) est sain et testé. En revanche, trois fondations produisent des bugs que les fixtures ne voient pas :

1. la construction et le parcours de l'arbre sont codés en dur, profondeur par profondeur ;
2. la résolution des attributs passe par plusieurs chaînes différentes ;
3. l'enchaînement des commentaires Outlook est émulé par un état mutable, alors que MJML le fait en post-traitement.

Les numéros de ligne ont été relevés pendant l'analyse, avant les commits `608c260` et `ef6905d`. Ils ont pu bouger. Les noms de fonctions font foi.

## Problèmes, par impact

### P1. L'arbre est construit en dur, par niveau d'imbrication (vérifié, corrigé à l'étape 1 sauf mj-wrapper-hero)

`mjml/component.go` : `createMJMLComponent`, `processComponentChildren` et `processSectionChildren` ne savent construire que les formes suivantes :

- body > section > column > contenu ;
- body > wrapper > section ;
- body > hero ;
- section > group > column.

Il n'y a pas de récursion générique. Résultats comparés à MJML 4.18 :

| Entrée | MJML 4.18 | safe-mjml |
|---|---|---|
| mj-hero > mj-social | éléments rendus | éléments perdus |
| mj-hero > mj-navbar | liens rendus | liens perdus |
| mj-hero > mj-accordion | éléments rendus | éléments perdus |
| mj-wrapper > mj-hero | contenu du hero rendu | contenu perdu |
| mj-hero > mj-carousel | rendu | erreur « mj-carousel requires at least one mj-carousel-image child » |

Les erreurs de `CreateComponent` sont ignorées en silence (`if err == nil`).

**Cause de fond :** l'interface `Component` (`mjml/components/base.go`) n'expose pas les enfants. Chaque passe refait donc son propre type switch, et ces listes divergent :

- `collectCarouselCSSFromComponent` (render.go) ;
- `prepareBodySiblings` (render.go) ;
- `collectColumnClassesFromComponent` (render.go) ;
- `has*Components` (render.go) ;
- `collectMetadataFromComponent` (render_metadata.go) ;
- la construction de l'arbre elle-même.

Par exemple, MJHeroComponent manque dans `prepareBodySiblings` et dans `collectColumnClasses`.

**État après l'étape 1 (branche refactor/step1-component-tree) :**

- Fait :
  - `GetChildren()` et `AddChild()` dans l'interface `Component` ;
  - un seul constructeur récursif, `buildChildren`, piloté par `componentContainers`, l'ensemble des balises qui ont des enfants composants dans MJML 4.18 ;
  - un seul parcours, `components.Walk`, qui ne suit que les enfants rendus et que toutes les passes de render.go et render_metadata.go utilisent ;
  - les balises inconnues sont ignorées explicitement, comme dans MJML ; les autres erreurs de `CreateComponent` arrêtent la construction.
- Résultat : hero > social, hero > navbar, hero > accordion et hero > carousel correspondent à MJML 4.18 et sont dans TestMJMLAgainstExpected.
- Reste : **mj-wrapper > mj-hero**. Le contenu correspond, mais le wrapper ajoute une table Outlook autour du hero, comme pour une section, en plus de celle que le hero écrit lui-même. C'est un défaut de P3 ; le cas `mj-wrapper-hero` reste dans TestMJMLKnownFailures.
- Non fait, décision de Pierre (2026-11-11) : signaler les balises inconnues et les composants mal placés par un canal d'avertissements séparé, pas par l'erreur de `Render` (Notifuse traite toute erreur comme un échec de compilation). safe-mjml-32 s'en charge.

### P2. La résolution des attributs est fragmentée (vérifié, bugs réels, corrigé)

**Fait le 2026-11-11** sur `refactor/step3-attributes` (étape 3) :

- Chaque composant fusionne ses attributs une fois, à sa construction : mj-attributes de son tag, puis ses mj-class (dont les `css-class` s'ajoutent), puis l'élément.
- Un parent transmet des attributs par le contexte (`ChildContext.PassedAttributes`), comme le `renderChildren` de MJML : mj-accordion et mj-accordion-element leurs bordures et icônes, mj-social ses attributs de texte et d'icône et son `inner-padding` comme `padding` des éléments.
- `resolveAttribute` lit les couches dans l'ordre de MJML, puis la valeur par défaut. Un attribut présent mais vide reste vide.
- Les résolveurs propres disparaissent (`resolveAttributeFromParent`, `InheritFromParent`, les huit `getAttribute`, `GetAttributeWithDefault`, `GetAttributeFast`), ainsi que `FontTracker`, que personne ne lisait.
- `render_metadata.go` n'est plus qu'un parcours : chaque composant qui écrit une police la déclare (`fontFamilies`).
- Les lectures directes de l'élément (`Node.GetAttribute`) passent par les couches, sauf la `font-family` explicite des titres et textes d'accordéon, que MJML lit dans `rawAttrs`.
- Les trois essais ci-dessous sont dans le cas `mj-attributes-layers`.
- Matrice : sur les 177 échecs connus de main (39 de la matrice des attributs, 138 des documents `enum-*`), 124 correspondent désormais à MJML (30 et 94). Il en reste 53, tous antérieurs à l'étape 3 : 44 documents `enum-*`, chacun avec sa raison, et 9 de la matrice des attributs : `column-width-empty` et `table-width-empty` (MJML y écrit NaN, que safe-mjml ne reproduit pas), `image-max-height-tag` et les six `wrapper-border-*` (étape 5).
- Les balises écrites à l'intérieur d'un `mj-class` (`<mj-class name="x"><mj-text color="red" /></mj-class>`, les `classesDefault` de mjml-core) donnent leurs attributs aux descendants de l'élément le plus proche qui porte la classe : au-dessus de mj-attributes et du mj-class de l'élément, sous l'élément lui-même. Fait après la v0.17.0, comme Pierre l'avait décidé (cas `mj-class-tag-defaults`).


`BaseComponent.ResolveAttribute` suit l'ordre élément > mj-class > global > défaut. Plusieurs composants la contournent :

- `MJAccordionElementComponent.getAttribute` (accordion.go) : élément > attribut brut du parent > défaut. Il saute mj-class, les mj-attributes de l'élément et les valeurs résolues du parent.
- `MJSocialElementComponent.resolveAttribute` (social.go) : une chaîne en 6 étapes.
- `BaseComponent.GetAttribute` saute les attributs globaux.

Essais comparés à MJML 4.18 : MJML applique les trois cas suivants, safe-mjml n'en applique aucun.

- `<mj-attributes><mj-accordion-element icon-height="77px">` ;
- un `mj-class` sur mj-accordion-element (icon-width) ;
- `<mj-accordion icon-height="66px">` dans mj-attributes, censé passer aux éléments.

**Référence MJML :** `mjml-core/lib/createComponent.js` et `index.js` fusionnent les attributs une seule fois, à la construction, dans cet ordre :

defaults < mj-all < attributs passés par le parent < tag < classes < élément

(`applyAttributes` fusionne tag, classes et élément sur le nœud ; `createComponent` place ensuite les attributs passés par le parent sous ceux du nœud, puis mj-all et les valeurs par défaut en dessous.) Ensuite, `getAttribute` n'est qu'une lecture. Dans safe-mjml, `render_metadata.go` doit reproduire chaque règle à la main.

### P3. L'enchaînement MSO repose sur un état mutable, pas sur le post-traitement de MJML (vérifié, corrigé)

**Protocole wrapper↔section supprimé** (étape 5, branche `refactor/step5-wrapper-on-section`). Le wrapper est bâti sur la section, comme mjml-wrapper étend mjml-section :
- un même rendu (`sectionBlock` : render, renderBefore, renderSection, renderAfter, renderWithBackground, renderFullWidth) sert les deux ;
- seul diffère `renderWrappedChildren` : une ligne Outlook et une cellule par colonne ou group pour la section, une ligne et une cellule par enfant pour le wrapper ;
- chaque section écrit sa propre table Outlook, et la fusion finale des conditionnels fait le reste ;
- le `gap` et l'index de chaque enfant passent par le contexte, comme `context.gap` et `props.index` dans MJML ;
- `SetWrapperMSOBackground`, `ConsumedWrapperMSOTable`, `wrapperMSOClosed`, `SetWrapperGap`, `InsideWrapper` et quatorze fonctions de `html/mso.go` disparaissent ;
- `mj-wrapper-section-background-url`, `mj-wrapper-hero`, `mj-wrapper-fractional-body-width` et sept documents de la matrice (les six `wrapper-border-*` et `enum-wrapper-full-width-empty-over-all`) correspondent désormais à MJML.


**Fait le 2026-11-11** sur `integration/mjml-4.18-refactor` : le document entier passe par `html.MergeOutlookConditionals`, comme `mergeOutlookConditionnals` de MJML, et `PendingMSOSectionClose`, `RemainingBodySections`/`NextBlockContinuesMSO` ainsi que les variantes « continuation » sont supprimés, avec une sortie identique octet pour octet. Reste le protocole wrapper↔section (`SetWrapperMSOBackground`, `ConsumedWrapperMSOTable`, `wrapperMSOClosed`), qui partira quand le wrapper sera bâti sur la section (étape 5), et l'échec connu `mj-wrapper-hero`.

MJML émet chaque bloc avec ses propres conditionnels, puis traite tout le HTML :

- `mergeOutlookConditionnals` remplace `/(<!\[endif]-->\s*?<!--\[if mso \| IE]>)/gm` par `''` ;
- `minifyOutlookConditionnals` (voir `mjml-core/lib/index.js`).

safe-mjml n'a aucun équivalent. Il émule la fusion pendant le rendu avec :

- `RenderOpts.PendingMSOSectionClose` et `RemainingBodySections` (body.go, section.go, wrapper.go) ;
- le protocole wrapper↔section : `wrapperMSOClosed`, `SetWrapperMSOBackground`, `SetWrapperGap`, `ConsumedWrapperMSOTable` ;
- les variantes « customSplitWrapper » et « continueMSOComment » de `MJSectionComponent.Render`, qui fait environ 650 lignes.

**Piège :** le wrapper construit ses enfants avec une copie `wrapperChildOpts := *opts`. Les sections d'un wrapper écrivent donc dans une copie de l'état.

safe-mjml-32 a confirmé ce point par deux bugs, corrigés depuis dans `608c260` :

- un mj-raw entre deux sections se retrouvait dans le conditionnel Outlook ;
- le tableau extérieur d'un wrapper full-width placé après une section aussi.

### P4. Le wrapper ignore la largeur du body (vérifié, corrigé pour le wrapper)

**Corrigé le 2026-11-11** sur `fix/mjml-4.18-parity` : le wrapper utilise sa largeur de conteneur, c'est-à-dire la largeur du body que `prepareBodySiblings` lui transmet, au lieu de la constante, VML compris. Les sections la recevaient déjà correctement. `mj-body-width-wrapper`, qui était un échec connu dans l'étape 0, passe désormais. Reste vrai : `BaseComponent.GetEffectiveWidth()` retombe sur 600 quand rien n'a transmis de largeur, ce que traitera l'étape 2 (contexte descendant).

`wrapper.go` utilise partout `GetDefaultBodyWidth()` et `GetDefaultBodyWidthPixels()`, une constante à 600px. `BaseComponent.GetEffectiveWidth()` retombe aussi sur 600 quand ContainerWidth vaut 0.

Essai avec `<mj-body width="500px">`, un wrapper et une section :

- MJML : 8 occurrences de 500, aucune de 600 ;
- safe-mjml : 7 occurrences de 600, une seule de 500.

MJML transmet `containerWidth` par `getChildContext`.

### P5. L'API publique est incohérente (vérifié)

- `RenderFromAST` et `NewFromAST` ne chargent pas les `GlobalAttributes` de mj-head. Essai : `<mj-text color="#ff0000">` dans mj-attributes s'applique via `Render`, pas via `RenderFromAST`.
- `Render` applique un post-patch par chaînes, `normalizeGroupColumnClassOrder`, que `RenderWithAST` n'applique pas. Sur un mj-group, les deux ne produisent donc pas le même HTML. L'ordre des classes devrait être juste à la source, dans group et column.
- À vérifier : le code d'appel de Notifuse utilise-t-il `RenderFromAST` ?

### P6. Mutations pendant le rendu (vérifié dans le code, en grande partie corrigé à l'étape 2)

- `group.go` écrit `columnComp.Attrs["mobile-width"]` pendant Render. `prepareBodySiblings` écrit `Attrs["width"]` sur les colonnes d'un group.
- group.go et hero.go remplacent le `RenderOpts` des enfants par une copie pendant Render. Le hero ne le fait que pour text, button et image, et `InsideHero` n'est lu nulle part.
- `parentAccordion` et `parentSocial` ne sont attachés que pendant Render. `render_metadata.go` doit donc rappeler `InheritFromParent`.
- Un second rendu du même arbre donne aujourd'hui un résultat identique. Mais l'ordre des passes est une contrainte implicite.

**État après l'étape 2 :** les largeurs, le nombre de frères et les indicateurs InsideWrapper, InsideGroup et InSection sont dans un `ChildContext` posé une fois, à la construction de l'arbre. prepareBodySiblings, les setters de largeur et de frères, les copies de RenderOpts (wrapper, group, hero), l'écriture de `width` et `mobile-width` dans les Attrs des colonnes de group et `msoCellFromParent` ont disparu. `parentAccordion` est posé par AddChild depuis la correction de la quatrième review. Restent des réglages faits pendant Render : `InheritFromParent`, `SetVerticalMode` et `SetMSOConditionalWrap` de mj-social, `SetWrapperGap` et `SetWrapperMSOBackground` du wrapper vers ses sections.

### P7. L'oracle des tests est laxiste (vérifié)

- `normalizeCSSContent` (integration_test.go) compare le contenu des `<style>` comme un multiset de caractères triés. Une inversion de règles ou de valeurs passe donc. Par exemple, `.a{width:50%}.b{width:100%}` et l'inverse sont jugés identiques.
- Les attributs `style=""` sont triés par propriété dans une map. L'ordre et les doublons de propriétés ne sont donc pas testés.
- Les fixtures sont minifiées par MJML : seules 2 sur 237 sont identiques octet pour octet à la sortie de safe-mjml.
- Conséquence : une refonte peut changer la sortie sans faire tomber de test. D'où l'étape 0.

### P8. Les identifiants du mode test sont globaux (vérifié dans le code, corrigé)

**Fait le 2026-11-11** par safe-mjml-3b (branche `refactor/p8-render-ids`, commits bd87dbd, e300079 et db6c5a5). Chaque rendu tire ses identifiants de carousel et de navbar d'un générateur propre (`mjml.WithIDGenerator`), aléatoire par défaut. Dans les tests, ce générateur rend les identifiants de la référence. Sur décision de Pierre, l'ancien mode test (`components.EnableTestMode`, le package `testmode`, les listes figées) est supprimé : c'est un changement cassant, sans effet sur Notifuse.

`carouselTestIDs` (carousel.go) et `navbarTestIDs` (navbar.go) sont des listes figées, consommées par un compteur atomique global dans l'ordre d'exécution des tests. Ajouter une fixture avant les autres décale tous les identifiants, et `-run` sur un seul test peut prendre le mauvais.

À la place : un générateur d'identifiants par rendu, injecté par une option, et un identifiant lu dans la fixture attendue.

### P9. Duplication (vérifié, coût de maintenance)

- wrapper.go : `renderFullWidthToWriter` et `renderSimpleToWriter` font environ 350 lignes chacun, et leur boucle sur les sections enfants est quasi identique. Dans MJML, mjml-wrapper fait 58 lignes et étend MjSection.
- social.go : le rendu de social-element est découpé par mode, vertical ou horizontal.
- `html/mso.go` fait 1018 lignes de fragments spécialisés.
- section.go écrit du HTML à la main en chaînes, à côté de HTMLTag.

### P10. Performance (mesurée, pas un motif de refonte)

Mesures avec `go test -bench . ./mjml` :

| Cas | Temps | Mémoire | Allocations |
|---|---|---|---|
| 100 sections | 3,5 ms | 4,7 Mo | 48,8 k |
| 100 sections, cache AST | 2,0 ms | | |
| 1000 sections | 35 ms (linéaire) | | |

L'analyse XML prend environ 45 % du temps. Fusionner les attributs une fois, à la construction (P2), réduirait probablement les allocations. Ce point n'est pas profilé.

### P11. Méthodes exportées de BaseComponent conservées par le linker (mesuré, taille du binaire)

Remonté par un utilisateur, qui l'a vu dans listmonk. Dès qu'un programme appelle une méthode par son nom via la réflexion (`reflect.Value.MethodByName`, ce que font `text/template` et `html/template` pour `{{.Foo}}`), le linker Go doit garder toutes les méthodes exportées des types qui peuvent y arriver. Les 33 méthodes exportées de `BaseComponent` sont promues dans les 31 types de composants qui l'embarquent.

Mesure du 2026-11-11, sur deux petits programmes qui appellent `mjml.Render`, le second exécutant aussi un template avec un appel de méthode (`go tool nm -size`) :

| | Symboles safe-mjml conservés | Code safe-mjml |
|---|---|---|
| sans appel par réflexion | 694 | 394 Ko |
| avec appel par réflexion | 2 602 | 550 Ko (+156 Ko) |

Environ 590 des quelque 1 900 symboles en plus sont des méthodes promues de `BaseComponent`. Seules 6 des 33 méthodes sont appelées hors du package `components` : `GetAttribute`, `GetDefaultAttribute`, `GetEffectiveWidth`, `SetContainerWidth`, `SetSiblings` et `SetRawSiblings`, la plupart à cause de l'interface `Component`. Les 27 autres peuvent devenir non exportées. C'est un changement cassant pour qui les appelle directement, acceptable en v0.

**Décision de Pierre :** le faire à la fin de l'étape 3, qui réécrit de toute façon la plupart de ces fonctions d'attributs. Il faudra refaire la mesure ensuite, avec la même sonde.

**Fait le 2026-11-11** à la fin de l'étape 3. `BaseComponent` n'exporte plus que les 7 méthodes de l'interface `Component`. Trois méthodes sans appelant sont supprimées (`GetAttributeAsColor`, `GetAttributeAsPixel`, `GetMSOClassAttribute`) et les autres deviennent non exportées. `GetAttribute` aussi : le package `mjml` lit désormais les attributs de `<mjml>`, mj-font et mj-style tels qu'ils sont écrits, comme MJML, et la couleur de fond de mj-body par `MJBodyComponent.BackgroundColor`. Nouvelle mesure, avec la même sonde :

| | sans appel par réflexion | avec appel par réflexion |
|---|---|---|
| v0.16.0 | 966 symboles, 423 Ko | 2 418 symboles, 560 Ko |
| étape 3 | 953 symboles, 425 Ko | 977 symboles, 428 Ko |

L'appel par réflexion ne coûte plus que 24 symboles et 3,6 Ko, au lieu de 1 452 symboles et 137 Ko.

### Écarts de parité trouvés en écrivant les cas rouges (hors refonte)

#### Hauteur d'un mj-hero sans height (corrigé le 2026-11-12)

**Corrigé sur `fix/hero-parity`**, avec les autres écarts du hero (arrière-plan écrit deux fois, valeurs vides, largeur fractionnaire, inner-*, border-radius, align, mode fluid-height) : la hauteur est `parseInt(height)` moins le padding, sans plancher, comme dans MJML ; `mj-hero-padding-no-height` est dans TestMJMLAgainstExpected. Restent, par la règle de Pierre (pas de NaN), une hauteur vide et un mode fluid-height sans ratio.

Décision de Pierre (2026-11-11) : à suivre, sans le corriger pour l'instant.

Un mj-hero sans height mais avec un padding vertical donne `height:0px` et `height="0"` dans safe-mjml. MJML 4.18 écrit la hauteur par défaut (0px) moins le padding, soit `height:-40px` et `height="-40"` pour 20px de padding. Dans `mjml/components/hero.go`, `Render` saute la soustraction quand la hauteur vaut 0px, et `calculateEffectiveHeight` borne le résultat à 0.

Le cas rouge `mj-hero-padding-no-height` le reproduit (TestMJMLKnownFailures). Les fixtures des cas hero de P1 n'ont plus de padding vertical, pour ne pas mêler cet écart.

#### Largeur des enfants d'un mj-hero (corrigé)

Décision de Pierre (2026-11-11) : à corriger tout de suite.

MJML donne aux enfants du hero sa largeur moins son padding gauche et droit (getChildContext de mjml-hero) ; safe-mjml leur donnait toute la largeur. Corrigé : `MJHeroComponent.ChildContainerWidth` retire le padding lu comme getShorthandAttrValue de mjml-core (`BaseComponent.shorthandAttrValue`). Couvert par `mj-hero-padding-children`.

#### Largeur des images de mj-carousel (corrigé)

Les images du carousel, et l'image de repli Outlook, avaient une largeur fixe de 600px. Elles prennent maintenant la largeur du conteneur du carousel, comme mj-carousel-image dans MJML. Couvert par `mj-hero-padding-carousel`.

Dans le même mouvement, la hauteur du hero (height moins le padding haut et bas) lit maintenant le padding comme MJML, y compris en 3 et 4 valeurs. Couvert par `mj-hero-height-padding-shorthand`.

Limite connue : un padding aussi large que le hero donne aux enfants une largeur de 0 ou moins, que safe-mjml lit comme « non définie » ; les enfants reçoivent alors la largeur par défaut, là où MJML écrit 0 ou une largeur négative.

#### Largeur des enfants d'une mj-column (corrigé à l'étape 2)

mj-column donnait à ses enfants sa largeur moins un padding lu seulement en 1 et 2 valeurs, sans padding-left ni padding-right, ni bordures, ni inner-border. Elle calcule maintenant getChildContext de mjml-column (`calculateEffectiveContentWidth`), écrit la largeur de sa cellule Outlook sans l'arrondir (199.98px) et donne à la table intérieure d'une colonne avec gutter ses inner-border. mj-image calcule sa largeur avec `boxWidth` (getBoxWidths) et écrit ses bordures par côté. Couvert par `mj-column-padding-children`, `mj-column-box-children` et `mj-image-box-width`.

#### Styles du head de plusieurs composants (suivi, non corrigé)

safe-mjml écrit le style de chaque composant dans un `<style>` à part (media query mobile de mj-image, accordéon, navbar, carousel), dans un ordre fixe. MJML les réunit dans un seul `<style>` : d'abord les styles par instance (carousel), puis les styles par composant (mj-image…), dans l'ordre où les composants apparaissent (buildStyleFromComponents de mjml-core). Un document qui a à la fois un carousel et un mj-image diffère donc de MJML. Le cas rouge `mj-carousel-image-head-styles` le reproduit (TestMJMLKnownFailures).

#### Écarts plus anciens relevés par la relecture de v0.16.0 (suivis ici, sans cas de test)

Présents dans v0.15.1 et v0.16.0, différents de MJML 4.18 (entrées dans le rapport de la relecture) :
- `mj-group vertical-align` (aussi via mj-all) n'est écrit ni sur le div du group ni sur sa cellule Outlook (`group.go`).
- Un `mj-raw` en premier enfant d'un group était écrit avant le `<table><tr>` Outlook au lieu d'après : corrigé avec les commentaires HTML (le group écrit sa table et sa ligne à part, comme mjml-group). Le même écart dans mj-wrapper est corrigé de la même façon : le wrapper écrit sa table Outlook, puis les raw qui précèdent sa première section, puis la ligne de celle-ci (cas `mj-wrapper-raw-first`).
- `mj-carousel padding` n'est pas écrit sur la cellule.
- `tb-width="33.3px"` donnait `width="33.3"` à la vignette, MJML `width="33"` : corrigé à l'étape 3 (`parseIntAttribute`).
- `mj-divider padding="5%"` : largeur Outlook 600px au lieu de 590px.
- `mj-button width="200px" border="solid 3px red"` : lien de 150px au lieu de 144px.
- `mj-column width="20em"` : part égale au lieu de `mj-column-px-20` et 20px ; body en `em`, `%` ou vide : 600px au lieu de la valeur telle quelle ; body sans unité (`600`) : safe-mjml ajoute `px`.
- Largeurs inhabituelles lues avec `strconv.ParseFloat` au lieu de parseInt/parseFloat de JavaScript : `33,33%`, `inf%`, `1e3px`, `1e-7%`.
- Ordre des déclarations de bordure de la table intérieure d'une colonne à gouttière (`styles/helpers.go`) : corrigé à l'étape 3 (`addTableStyle`, cas `mj-column-border-order`).

#### Écarts trouvés par la revue de l'étape 3 (suivis, non corrigés)

Antérieurs à l'étape 3, confirmés contre MJML 4.18 :

- **Suffixes de classe avec plusieurs classes.** `section.go`, `group.go`, `navbar.go`, `wrapper.go`, `carousel.go` ajoutent le suffixe (`-outlook`, `-thumbnail`) à toute la chaîne. Exemple : `cx cy-outlook` au lieu de `cx-outlook cy-outlook`.
- **`align` de mj-all.** La section lisait `align` pour sa table Outlook, où MJML écrit toujours `center`, et la cellule Outlook de la colonne ne recevait pas l'`align` de la colonne : corrigé à la fin de l'étape 3, que le balayage des couches avait fait apparaître (cas `mj-all-align-padding-empty`).
- **Police par défaut.** L'heuristique `importDefault` (`render.go`) n'a plus d'effet que là où elle se trompe : `<mj-text font-family="">` importe Ubuntu, alors que MJML n'importe rien.
- **Largeurs négatives.** Une largeur négative est écrite à 0 (choix documenté) là où MJML écrit une valeur négative.
- **Liens du navbar.** safe-mjml joint `base-url` et `href` avec exactement un `/`. MJML 4.18 les concatène : `https://example.com` et `page` donnent `https://example.compage`. **Décision de Pierre (2026-11-11) :** garder le `/` de safe-mjml, pour ne pas changer les liens des gabarits existants.
- **Largeurs NaN.** MJML écrit `width:NaN%` pour une largeur de colonne vide (`column-width-empty`) et `width="NaN"` pour une largeur de mj-table vide (`table-width-empty`). **Décision de Pierre (2026-11-11) :** ne pas reproduire NaN. La colonne garde sa part égale, la table sa largeur telle qu'elle est écrite (`parseIntAttribute`). TestRenderInvariants refuse toute largeur NaN.
- **Vus en passant :**
  - `<mj-image src="">` faisait échouer tout le rendu, alors que MJML rend l'image : corrigé à la fin de l'étape 3, `src` absent compris (cas `mj-image-src-missing-or-empty` et `mj-image-no-src`).
  - `thumbnails="supported"` supprime les vignettes, et toute valeur non vide de `hamburger` en affiche un.
  - Plusieurs attributs sont ignorés : le padding de mj-navbar et de mj-carousel, `padding-top/bottom/left` de mj-accordion, le `font-weight` des titres et textes d'accordéon, et `rel` sur les images de carousel.
  - safe-mjml importe Montserrat automatiquement et ignore Droid Sans ; MJML fait l'inverse.

#### Écarts relevés par la relecture de l'oracle (2026-11-11)

- Les commentaires HTML du template sont supprimés ; MJML les garde (keepComments). Échec connu `mj-body`.
- `mj-accordion` ne lit que `padding` et `padding-right` (confié à l'étape 3).
- Le parser décode `&lt;` et `&gt;` du contenu avant l'analyse : `<mj-button>B1 &lt;x&gt;</mj-button>` échoue, `<mj-text>Use &lt;v:rect&gt;</mj-text>` écrit un `<v:rect>` actif. MJML garde les entités.
- `<mj-raw><![if !mso]>…<![endif]></mj-raw>` est une erreur d'analyse dans safe-mjml ; MJML le rend.
- `testutils.EffectiveDeclarations` ne gère pas `!important`, la casse des propriétés, `flex`, `outline-offset`, `border-image`, les raccourcis qui se chevauchent (`border-top` et `border-color`) ni un `;` dans `url(data:…)`. safe-mjml n'émet aucun de ces cas aujourd'hui.

## Plan par étapes

0. **Filet de sécurité** (S, une demi-journée)
   - Ajouter un test snapshot octet pour octet de la sortie de safe-mjml, mis à jour seulement avec `-update`.
   - Durcir `normalizeCSSContent` pour comparer les règles CSS, plus les caractères triés. C'est une décision de Pierre.
   - Ajouter comme fixtures MJML 4.18 les cas de P1, P2 et P4 : hero>social, hero>navbar, hero>accordion, hero>carousel, wrapper>hero, body width + wrapper, attributs de l'accordéon. Elles seront rouges au départ.
1. **Arbre générique** (S-M) — fait, voir l'état sous P1
   - `GetChildren()` dans l'interface.
   - Un constructeur récursif piloté par une table des enfants autorisés par tag.
   - Un `Walk` partagé par toutes les passes.
   - Corrige P1 et supprime environ 300 lignes de switchs.
2. **Contexte descendant immuable** (S-M) — fait, voir l'état sous P6
   - Un `ChildContext` (containerWidth, insideWrapper, insideGroup, groupColumnCount, parent) remplace les copies de RenderOpts et les setters.
   - Corrige P4 et une partie de P6.
3. **Attributs fusionnés une fois, à la construction** (M) — fait, voir l'état sous P2 et P11
   - Les parents déclarent les attributs qu'ils transmettent.
   - Les résolveurs propres à chaque composant disparaissent.
   - `render_metadata.go` devient trivial.
   - Corrige P2.
   - Un attribut présent mais vide remplace la valeur par défaut, comme dans MJML : c'est la majorité des 39 échecs connus de la matrice (`mjml/testdata/matrix/known-failures.txt`).
4. **Post-traitement MSO comme MJML** (M-L, l'étape la plus risquée) — fait, voir P3
   - Chaque bloc émet ses conditionnels complets, puis merge et minify passent sur la sortie finale.
   - PendingMSOSectionClose, RemainingBodySections et le protocole wrapper↔section disparaissent.
   - Il faut bufferiser la sortie.
5. **Opportuniste**
   - **API publique minimale et paquets internes — fait le 2026-11-12 sur `refactor/step5-internal`.** Seul `mjml` reste public : `Render`, `Compile`, `Parse` et `Document.Render`, `NewCache` et `WithCache`, `WithKeepComments`, `WithIDGenerator`, `*mjml.Error` pour tout échec, `Warning`. Tous les autres paquets, `parser` compris, sont dans `internal/`. Les fonctions citées plus bas (`RenderFromAST`, `NewFromAST`, `CreateComponent`…) n'existent plus ; voir le README.
   - **Contenu brut des balises de contenu — fait le 2026-11-11 sur `fix/ending-tags-raw`.** Le parser enveloppe en CDATA le contenu de toutes les ending tags de mjml-core avant l'analyse et ne décode les entités qu'en dehors ; les composants écrivent ce contenu tel quel. Pour Notifuse, à son passage en v1 : un seul test à adapter, `TestCompileTemplateSubjectPreviewOverrideFallbackEscapesRenderedValue`, qui cherche `Tom & Jerry` brut alors que la prévisualisation garde maintenant `Tom &amp; Jerry &#60;weekly&#62;` (même texte affiché, et `<weekly>` n'est plus une balise). Les contournements de Notifuse (`escapeContent`, `escapeXMLContent` avec `&#60;`) restent corrects ; leurs commentaires sur le pré-décodage de safe-mjml deviennent caducs. **À changer dans Notifuse au passage en v1 (décision de Pierre, 2026-11-11) :** `escapeContent` échappe aussi le contenu de mj-style ; comme MJML, safe-mjml ne décode plus les entités dans mj-style, donc une règle CSS avec `>` (`&#62;`) ou `&` (`&amp;`) ne s'appliquerait plus. Notifuse doit cesser d'échapper mj-style en montant de version. Historique : MJML garde le contenu de toutes ses « ending tags » (mj-text, mj-button, mj-table, mj-raw, mj-navbar-link, mj-social-element, mj-accordion-title et -text) comme du HTML brut ; safe-mjml ne le fait que pour mj-text (CDATA) et passe les autres au parseur XML, après avoir décodé `&lt;` et `&gt;`. Conséquences : `&lt;x&gt;` dans un mj-button fait échouer tout le rendu, `&lt;v:rect&gt;` devient une balise active, et le HTML en ligne des autres balises perd ses balises ou leurs attributs (le href d'un lien dans un accordéon). Échecs connus `ending-tags-escaped-entities` et `ending-tags-inline-html`. À corriger dans le parser (contenu brut pour toutes les ending tags, entités conservées) et dans les composants qui réécrivent leurs enfants.
   - Le wrapper bâti sur section, ce qui supprime le protocole wrapper↔section — fait, voir P3.
   - mj-column (et mj-hero) écrivent le `<tr><td>` autour de chaque enfant, comme `renderColumn` de MJML, au lieu que chaque composant feuille écrive sa ligne — fait (`renderInCell`) : les échecs connus `mj-section-nested-in-column` et `validation-warnings` correspondent désormais à MJML.
   - Largeur des enfants de mj-column et lecture du padding de mj-image avec `boxWidth` (échec connu `mj-column-padding-children`), en même temps que l'étape 2.
   - À valider par Pierre : `MJMLComponent.AddChild` panique (ses enfants ne seraient jamais rendus), faute de valeur de retour dans l'interface.
   - Un seul rendu de social-element, paramétré par le mode.
   - L'ordre des classes de group corrigé à la source, puis `normalizeGroupColumnClassOrder` retiré.
   - P8.
   - P5, qui peut être fait tout de suite, indépendamment du reste.

Ordre conseillé : 0 → P5 et P8 → 1 → 2 → 3 → 4 → reste de 5. À chaque étape, les fixtures MJML et le snapshot restent verts. Un changement du snapshot n'est accepté que s'il s'explique par une correction voulue.

## Non vérifié

- Pas de profil pprof.
- Autres imbrications (mj-raw à différents niveaux) : non testées.
- Sûreté en concurrence des RenderOpts copiés par valeur : non testée.
- Que le post-traitement MSO reproduise réellement toutes les fixtures : c'est une hypothèse, fondée sur le fait que MJML procède ainsi.
- Le code d'appel de Notifuse.

Les essais ayant servi à l'analyse se trouvent dans le scratchpad de la session d'analyse : fichiers `cases/*.mjml`, rendus MJML `*.ref.html` et rendus safe-mjml `*.go.html`. Ce dossier est temporaire. À l'étape 0, ces cas sont à recréer dans `mjml/testdata`.

<div align="center">

<img src=".github/assets/banner-light.svg#gh-light-mode-only" alt="Roundbar | Un écran rond qui suit l'app au premier plan" width="100%">
<img src=".github/assets/banner-dark.svg#gh-dark-mode-only" alt="Roundbar | Un écran rond qui suit l'app au premier plan" width="100%">

<br>
<br>

<a href="https://www.espressif.com/en/products/socs/esp32-s3"><img src="https://img.shields.io/badge/ESP--IDF_6-4F378B?style=for-the-badge&logo=espressif&logoColor=D0BCFF" alt="ESP-IDF 6"></a>
<a href="https://lvgl.io"><img src="https://img.shields.io/badge/LVGL_9.3-4F378B?style=for-the-badge&logo=c&logoColor=D0BCFF" alt="LVGL 9.3"></a>
<a href="https://go.dev"><img src="https://img.shields.io/badge/Go-4F378B?style=for-the-badge&logo=go&logoColor=D0BCFF" alt="Go"></a>
<a href="https://vuejs.org"><img src="https://img.shields.io/badge/Vue_3-4F378B?style=for-the-badge&logo=vuedotjs&logoColor=D0BCFF" alt="Vue 3"></a>
<a href="https://m3.material.io"><img src="https://img.shields.io/badge/Material_You-4F378B?style=for-the-badge&logo=materialdesign&logoColor=D0BCFF" alt="Material You"></a>
<a href="https://www.gnome.org"><img src="https://img.shields.io/badge/GNOME_Shell-4F378B?style=for-the-badge&logo=gnome&logoColor=D0BCFF" alt="GNOME Shell"></a>

<img src="https://img.shields.io/badge/carte-ESP32--S3_360%C3%97360-4A4458?style=flat-square&logo=espressif&logoColor=white" alt="ESP32-S3 360×360">
<img src="https://img.shields.io/badge/bureau-GNOME_Wayland-633B48?style=flat-square&logo=linux&logoColor=white" alt="GNOME Wayland">
<img src="https://img.shields.io/badge/USB-clavier_%2B_souris-4F378B?style=flat-square" alt="Clavier et souris USB">
<img src="https://img.shields.io/badge/daemon-~0%25_CPU_%C2%B7_15_Mo-4A4458?style=flat-square" alt="Daemon ~0 % CPU, 15 Mo">
<img src="https://img.shields.io/badge/binaire-1_fichier-633B48?style=flat-square" alt="Un seul binaire">

<br>
<br>

<b>
<a href="#preview">Aperçu</a> ·
<a href="#features">Fonctionnalités</a> ·
<a href="#install">Installation</a> ·
<a href="#firmware">Firmware et carte</a> ·
<a href="#stack">Stack</a> ·
<a href="#dev">Développement</a>
</b>

</div>

<br>

> **Roundbar** est un contrôleur tactile rond à fabriquer soi-même, dans l'esprit du Flexbar : une
> **Guition JC3636W518C** (ESP32-S3, écran IPS tactile rond de 1,8" en 360×360) branchée en USB.
> Ses pages **changent toutes seules selon l'app au premier plan** sur un PC Linux sous GNOME (Wayland) :
> raccourcis, lecture en cours, jauges système, molettes, minuteur, mixeur et bien d'autres, en
> **Material 3 Expressive**. La carte se présente comme un **clavier et une souris USB**, donc les
> raccourcis marchent même sans le logiciel côté PC. Tout se règle dans un **configurateur web** avec
> un aperçu rond en direct.

<br>

<a name="preview"></a>
<img src=".github/assets/section-preview-light.svg#gh-light-mode-only" alt="01 | Aperçu" width="100%">
<img src=".github/assets/section-preview-dark.svg#gh-dark-mode-only" alt="01 | Aperçu" width="100%">

<table>
<tr>
<th width="25%">Lecture en cours</th>
<th width="25%">Raccourcis</th>
<th width="25%">Navigateur</th>
<th width="25%">Éditeur de code</th>
</tr>
<tr>
<td><img src=".github/assets/screens/device-media.webp" alt="Lecture en cours : pochette floutée en fond, précédent, lecture, suivant, volume sur l'arc du bord"></td>
<td><img src=".github/assets/screens/device-shortcuts.webp" alt="Page de raccourcis : huit boutons en anneau et un bouton central, formes Material 3 Expressive"></td>
<td><img src=".github/assets/screens/device-browser.webp" alt="Page du navigateur : onglets, historique, recherche, recharger"></td>
<td><img src=".github/assets/screens/device-code.webp" alt="Page de l'éditeur de code : terminal, enregistrer, rechercher, débogueur"></td>
</tr>
<tr>
<th>Chat vocal</th>
<th>Système</th>
<th>Horloge et météo</th>
<th>Minuteur</th>
</tr>
<tr>
<td><img src=".github/assets/screens/device-voice.webp" alt="Page d'une app de chat vocal : micro, casque, caméra"></td>
<td><img src=".github/assets/screens/device-stats.webp" alt="Jauges CPU, GPU, RAM et température du CPU"></td>
<td><img src=".github/assets/screens/device-clock.webp" alt="Heure, date, météo et une horloge mondiale"></td>
<td><img src=".github/assets/screens/device-timer.webp" alt="Minuteur de 25 minutes avec préréglages 5, 15 et 25"></td>
</tr>
<tr>
<th>Mixeur</th>
<th>Fenêtres</th>
<th>Molette de volume</th>
<th></th>
</tr>
<tr>
<td><img src=".github/assets/screens/device-mixer.webp" alt="Mixeur : une ligne de volume par app qui joue du son"></td>
<td><img src=".github/assets/screens/device-windows.webp" alt="Sélecteur de fenêtres : icônes des fenêtres ouvertes en anneau"></td>
<td><img src=".github/assets/screens/device-dial.webp" alt="Molette de volume sur l'arc du bord"></td>
<td></td>
</tr>
</table>

<div align="center">
<sub>Captures prises sur la vraie carte avec <code>roundbar screenshot out.png</code></sub>
</div>

<br>

<table>
<tr>
<th>Profils, thème sombre</th>
<th>Profils, thème clair</th>
</tr>
<tr>
<td><img src=".github/assets/screens/configurator-profiles-dark.webp" alt="Configurateur : liste des profils par app"></td>
<td><img src=".github/assets/screens/configurator-profiles-light.webp" alt="La même vue en thème clair"></td>
</tr>
<tr>
<th>Éditeur de page avec l'aperçu rond en direct</th>
<th>Le même éditeur en thème clair</th>
</tr>
<tr>
<td><img src=".github/assets/screens/configurator-editor-dark.webp" alt="Éditeur de page : boutons, formes, actions et aperçu rond dessiné comme sur la carte"></td>
<td><img src=".github/assets/screens/configurator-editor-light.webp" alt="L'éditeur de page en thème clair"></td>
</tr>
<tr>
<th>Apparence : couleur de base Material You</th>
<th>Apparence en thème clair</th>
</tr>
<tr>
<td><img src=".github/assets/screens/configurator-appearance-dark.webp" alt="Apparence : couleur de base, palette envoyée à la carte, aperçu rond"></td>
<td><img src=".github/assets/screens/configurator-appearance-light.webp" alt="Les réglages d'apparence en thème clair"></td>
</tr>
<tr>
<th>Appareil : état, écran, mise à jour du firmware</th>
<th>Appareil en thème clair</th>
</tr>
<tr>
<td><img src=".github/assets/screens/configurator-device-dark.webp" alt="Réglages de l'appareil : luminosité, veille, rotation, mise à jour du firmware"></td>
<td><img src=".github/assets/screens/configurator-device-light.webp" alt="Les réglages de l'appareil en thème clair"></td>
</tr>
</table>

<br>

<a name="features"></a>
<img src=".github/assets/section-features-light.svg#gh-light-mode-only" alt="02 | Fonctionnalités" width="100%">
<img src=".github/assets/section-features-dark.svg#gh-dark-mode-only" alt="02 | Fonctionnalités" width="100%">

<table>
<tr>
<td width="33%" valign="top">

### Profils par app
Chaque app a son **profil** et ses pages. La carte bascule **dès que le focus change**, et
retient la **dernière page vue** de chaque profil. La bascule automatique se met en pause depuis
les réglages rapides, avec un badge en haut de l'écran. Chaque profil peut avoir **sa propre couleur**.

</td>
<td width="33%" valign="top">

### Pages de raccourcis
Jusqu'à **8 boutons en anneau** et un bouton central, chacun avec sa forme **M3 Expressive**
(circle, square, cookie, clover, sunny, flower, burst) et une action au **tap** et à
l'**appui long**. Actions : touches (**AZERTY ou QWERTY**), texte, touches multimédia, saut de page,
commande shell, ouverture d'une app ou d'une URL, micro coupé, et **macros** avec un enregistreur
dans le configurateur.

</td>
<td width="33%" valign="top">

### Lecture en cours
Le lecteur MPRIS actif avec titre, artiste, **pochette floutée en fond** et des **couleurs tirées de
la pochette**. Précédent, lecture et suivant au centre, **volume sur l'arc du bord**.

</td>
</tr>
<tr>
<td valign="top">

### Système et molettes
Jauges **CPU, GPU, VRAM, RAM et températures** (AMD via sysfs), relevées seulement quand une page
de stats est à l'écran. Molettes de **volume**, de **micro**, de **luminosité** ou **personnalisées**,
qu'on tourne en glissant le long du bord.

</td>
<td valign="top">

### Horloge, minuteur, mixeur
Horloge analogique ou numérique avec la **météo** (Open-Meteo) et jusqu'à **3 horloges mondiales**.
**Minuteur** réglé à l'anneau, avec un mode **pomodoro**. **Mixeur** de volume par app,
**sélecteur de fenêtres** et **historique du presse-papiers** (les copies des gestionnaires de mots de
passe sont ignorées).

</td>
<td valign="top">

### Encore plus de pages
**Images** : diaporama de photos ou GIF animé. **Touchpad** : la carte devient une souris USB (tap et
appui long pour cliquer, glisser-déposer, anneau de défilement). **Discord** : ton salon vocal en direct
(membres, qui parle, qui est coupé) depuis Vesktop, via son port de débogage local, sans modifier
Vesktop ni Vencord ; ou via le RPC du client officiel, avec couper le micro, le son, quitter.

</td>
</tr>
<tr>
<td valign="top">

### Gestes et panneaux
On tire vers le bas pour les **réglages rapides** : sauts de page, **écran vraiment éteint** (réveil par
double tap) et pause de la bascule automatique. On tire vers le haut pour les **notifications** de GNOME.
Les pages **tournent en boucle**, et un **double tap sur le bord** change de page, ouvre les réglages rapides ou éteint l'écran, au choix.

</td>
<td valign="top">

### Suit le PC
L'écran s'éteint quand le PC s'éteint ou se met en veille et se rallume à son retour. **Rotation** de
l'écran au choix. Texte **japonais, chinois et coréen** et **emoji en couleur**, grâce à un pack de
glyphes de 9 Mo construit depuis les polices Noto du système.

</td>
<td valign="top">

### Léger et autonome
Le daemon réagit aux événements, sans scrutation : **~0 % de CPU et ~15 Mo de RAM** au repos.
La carte garde sa config en flash et se présente comme un **clavier et une souris USB** : les
raccourcis marchent même sans le logiciel côté PC.

</td>
</tr>
</table>

**Le configurateur.** Un aperçu rond en direct, dessiné avec **la même mise en page que la carte**, une
couleur de base **Material You**, le copier-coller de pages d'un profil à l'autre, un bouton de
**mise à jour du firmware** avec son log en direct, et des captures de l'écran de la carte avec
`roundbar screenshot out.png`.

<br>

<a name="install"></a>
<img src=".github/assets/section-install-light.svg#gh-light-mode-only" alt="03 | Installation" width="100%">
<img src=".github/assets/section-install-dark.svg#gh-dark-mode-only" alt="03 | Installation" width="100%">

Il faut Go, Node et npm, sur un bureau GNOME en Wayland :

```sh
make install
```

| Étape | Ce que fait `make install` |
| --- | --- |
| 1 | Compile le configurateur (Vue 3) et le binaire Go qui l'embarque |
| 2 | Installe `~/.local/bin/roundbar` |
| 3 | Installe et démarre le service systemd utilisateur `roundbar daemon` |
| 4 | Installe l'extension GNOME `roundbar@beedy` et le lanceur d'app |

Déconnecte-toi puis reconnecte-toi **une fois** pour que GNOME charge l'extension. Ouvre ensuite
**Roundbar** depuis la grille d'apps, ou lance `roundbar config`.

| Commande | Rôle |
| --- | --- |
| `roundbar config` | Ouvre le configurateur dans le navigateur (`--no-open` pour seulement afficher l'URL) |
| `roundbar status` | État du daemon et de la carte |
| `roundbar screenshot out.png` | Enregistre ce qu'affiche la carte en PNG |
| `roundbar bootloader` | Redémarre la carte en mode téléchargement USB, pour la flasher |
| `roundbar daemon` | Le pont entre la carte et le bureau (lancé par le service systemd) |
| `roundbar install` | Installe le service, l'extension GNOME et le lanceur |

<br>

<a name="firmware"></a>
<img src=".github/assets/section-firmware-light.svg#gh-light-mode-only" alt="04 | Firmware et carte" width="100%">
<img src=".github/assets/section-firmware-dark.svg#gh-dark-mode-only" alt="04 | Firmware et carte" width="100%">

Le firmware demande **ESP-IDF v6**, installé dans `~/esp/esp-idf-v6.0.3` :

```sh
. ~/esp/esp-idf-v6.0.3/export.sh
```

Pour accéder au port série sans root, installe la règle udev :

```sh
sudo cp deploy/70-roundbar.rules /etc/udev/rules.d/ && sudo udevadm control --reload && sudo udevadm trigger -s tty
```

Puis compile et flashe :

```sh
make flash
```

`make flash` (ou `tools/flash.sh`) redémarre la carte **en mode téléchargement tout seul**, sans
toucher au bouton BOOT, et n'écrit le pack de polices que **s'il a changé**. Si la carte ne fait pas
encore tourner Roundbar : maintiens BOOT en la branchant. Une fois installé, le bouton de mise à jour
du configurateur fait la même chose.

<table>
<tr>
<td width="50%" valign="top">

**Carte**
- Guition **JC3636W518C**, ESP32-S3
- Écran IPS rond **1,8"**, 360×360, tactile
- **16 Mo** de flash, **8 Mo** de PSRAM
- USB : clavier et souris HID, plus un lien série CDC (lignes JSON)

</td>
<td width="50%" valign="top">

**Broches**
- LCD QSPI : SCK **9**, D0–D3 **11–14**, CS **10**, RST **47**, rétroéclairage **15**
- Tactile I²C (CST816) : SDA **7**, SCL **8**, INT **41**, RST **40**

</td>
</tr>
</table>

<br>

<a name="stack"></a>
<img src=".github/assets/section-stack-light.svg#gh-light-mode-only" alt="05 | Stack" width="100%">
<img src=".github/assets/section-stack-dark.svg#gh-dark-mode-only" alt="05 | Stack" width="100%">

<table>
<tr>
<th align="left">Dossier</th>
<th align="left">Contenu</th>
</tr>
<tr><td><code>docs/spec.md</code></td><td>Le contrat entre toutes les parties : config, protocole série, socket du daemon, API du configurateur.</td></tr>
<tr><td><code>firmware/</code></td><td>ESP-IDF 6 + LVGL 9.3 en C : pages Material 3 Expressive sur l'écran rond, clavier et souris USB, lien série CDC.</td></tr>
<tr><td><code>cmd/roundbar</code></td><td>Point d'entrée : <code>daemon</code>, <code>config</code>, <code>status</code>, <code>screenshot</code>, <code>bootloader</code>, <code>install</code>.</td></tr>
<tr><td><code>internal/config</code></td><td>Lecture, validation et surveillance (inotify) de <code>~/.config/roundbar/config.json</code>.</td></tr>
<tr><td><code>internal/daemon</code></td><td>Le pont événementiel entre la carte et le bureau.</td></tr>
<tr><td><code>internal/device</code></td><td>Connexion USB CDC et messages JSON avec la carte.</td></tr>
<tr><td><code>internal/desktop</code></td><td>Focus, fenêtres, notifications et presse-papiers via l'extension GNOME (D-Bus), MPRIS, actions.</td></tr>
<tr><td><code>internal/audio</code></td><td>Volume, micro et mixeur par app (PipeWire).</td></tr>
<tr><td><code>internal/stats</code></td><td>CPU, RAM, GPU et températures lus dans <code>/proc</code>, hwmon et amdgpu.</td></tr>
<tr><td><code>internal/cover</code></td><td>Pochettes : fond flouté et palette tirée de l'image.</td></tr>
<tr><td><code>internal/assets</code></td><td>Images des pages image, rendues au format de la carte.</td></tr>
<tr><td><code>internal/icons</code></td><td>Icônes des apps envoyées à la carte.</td></tr>
<tr><td><code>internal/discord</code></td><td>Salon vocal en direct : Vesktop par le protocole DevTools local (<code>~/.config/vesktop-flags.conf</code>), ou le RPC du Discord officiel.</td></tr>
<tr><td><code>internal/configui</code></td><td>Serveur HTTP du configurateur sur 127.0.0.1, qui s'arrête après la fermeture du dernier onglet.</td></tr>
<tr><td><code>internal/ipc</code></td><td>Socket du daemon (<code>$XDG_RUNTIME_DIR/roundbar.sock</code>).</td></tr>
<tr><td><code>web/</code></td><td>Configurateur Vue 3, Material 3 Expressive, palette Material You générée depuis une couleur de base, embarqué dans le binaire.</td></tr>
<tr><td><code>gnome-extension/</code></td><td><code>roundbar@beedy</code> : publie le focus, les fenêtres, les notifications et le presse-papiers sur D-Bus, par signaux uniquement, sans scrutation. Sous GNOME Wayland, c'est le seul moyen de connaître la fenêtre active.</td></tr>
<tr><td><code>shared/icons.json</code></td><td>Le sous-ensemble de Material Symbols Rounded intégré au firmware.</td></tr>
<tr><td><code>tools/</code></td><td><code>flash.sh</code>, <code>fontpack</code> (pack de glyphes CJK et emoji), <code>gen-icons.py</code>, <code>readme-assets.py</code>.</td></tr>
<tr><td><code>deploy/</code></td><td>Règle udev <code>70-roundbar.rules</code>.</td></tr>
</table>

<br>

<a name="dev"></a>
<img src=".github/assets/section-dev-light.svg#gh-light-mode-only" alt="06 | Développement" width="100%">
<img src=".github/assets/section-dev-dark.svg#gh-dark-mode-only" alt="06 | Développement" width="100%">

```sh
make                                              # configurateur + binaire Go dans bin/roundbar
make test                                         # go test ./...
make firmware                                     # idf.py build, ESP-IDF chargé
make flash                                        # compile et flashe la carte
go run ./tools/fontpack -out firmware/build/fonts.bin   # reconstruit le pack de glyphes
```

<details>
<summary><b>Notes</b></summary>

<br>

- `docs/spec.md` se modifie **en premier**, puis le code de chaque partie.
- La config vit dans `~/.config/roundbar/config.json`. Le configurateur l'écrit, le daemon la surveille et
  la pousse à la carte, qui en garde une copie en flash.
- Le pack de glyphes est construit depuis Noto Sans CJK et Noto Color Emoji installés sur le système.
  `FORCE_FONTS=1 make flash` le réécrit même s'il n'a pas changé.
- Les captures de la carte se prennent avec `roundbar screenshot out.png`.

Les visuels de ce README se régénèrent avec `python3 tools/readme-assets.py`.

</details>

<br>

<img src=".github/assets/footer-light.svg#gh-light-mode-only" alt="beedywool.com" width="100%">
<img src=".github/assets/footer-dark.svg#gh-dark-mode-only" alt="beedywool.com" width="100%">

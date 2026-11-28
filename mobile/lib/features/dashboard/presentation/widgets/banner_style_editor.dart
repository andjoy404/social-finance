import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/banner_crop_image.dart';

const List<Map<String, String>> neonColorPresets = [
  {'name': 'Neon Purple', 'hex': '#A970FF'},
  {'name': 'Electric Cyan', 'hex': '#00F0FF'},
  {'name': 'Neon Pink', 'hex': '#FF2A85'},
  {'name': 'Neon Green', 'hex': '#00E599'},
  {'name': 'Neon Amber', 'hex': '#FFB800'},
  {'name': 'Sky Blue', 'hex': '#38BDF8'},
  {'name': 'Indigo Glow', 'hex': '#6366F1'},
  {'name': 'Coral Flare', 'hex': '#FF6B6B'},
];

Color parseHexColor(String hex) {
  var clean = hex.replaceAll('#', '').trim();
  if (clean.length == 3) {
    clean = '${clean[0]}${clean[0]}${clean[1]}${clean[1]}${clean[2]}${clean[2]}';
  }
  if (clean.length == 6) {
    clean = 'FF$clean';
  }
  return Color(int.tryParse(clean, radix: 16) ?? 0xFFA970FF);
}

GradientConfig computeNeonGradientConfig(String hex) {
  final col = parseHexColor(hex);
  final toR = (col.r * 255 * 0.18).clamp(12, 255).toInt();
  final toG = (col.g * 255 * 0.18).clamp(8, 255).toInt();
  final toB = (col.b * 255 * 0.18).clamp(20, 255).toInt();
  final toHex =
      '#${toR.toRadixString(16).padLeft(2, '0')}${toG.toRadixString(16).padLeft(2, '0')}${toB.toRadixString(16).padLeft(2, '0')}'
          .toUpperCase();

  return GradientConfig(
    from: hex.toUpperCase(),
    to: toHex,
    angle: 135,
    glow: true,
  );
}

/// Multi-layer smooth ambient neon glow background for banners.
class BannerNeonGlowBackground extends StatelessWidget {
  final Color color;

  const BannerNeonGlowBackground({
    super.key,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        // Base dark canvas
        Container(color: const Color(0xFF0C0916)),
        // Layer 1: Soft diagonal ambient wash
        Container(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [
                color.withValues(alpha: 0.22),
                const Color(0xFF0C0916).withValues(alpha: 0.95),
              ],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
          ),
        ),
        // Layer 2: Primary broad diffused neon bloom at top-right
        Container(
          decoration: BoxDecoration(
            gradient: RadialGradient(
              center: const Alignment(0.55, -0.55),
              radius: 0.95,
              colors: [
                color.withValues(alpha: 0.65),
                color.withValues(alpha: 0.25),
                Colors.transparent,
              ],
              stops: const [0.0, 0.40, 1.0],
            ),
          ),
        ),
        // Layer 3: Secondary subtle reflection at bottom-left
        Container(
          decoration: BoxDecoration(
            gradient: RadialGradient(
              center: const Alignment(-0.7, 0.7),
              radius: 0.8,
              colors: [
                color.withValues(alpha: 0.22),
                Colors.transparent,
              ],
              stops: const [0.0, 1.0],
            ),
          ),
        ),
        // Layer 4: Content contrast scrim (ensures text and badge readability)
        Container(
          decoration: const BoxDecoration(
            gradient: LinearGradient(
              colors: [
                Color(0xB3000000), // 70% black
                Color(0x33000000), // 20% black
                Colors.transparent,
              ],
              begin: Alignment.bottomCenter,
              end: Alignment.topCenter,
              stops: [0.0, 0.45, 1.0],
            ),
          ),
        ),
      ],
    );
  }
}

class BannerStyleEditor extends ConsumerStatefulWidget {
  final BannerStyleConfig? value;
  final ValueChanged<BannerStyleConfig> onChanged;
  final String titlePreview;
  final String bodyPreview;
  final String category;

  const BannerStyleEditor({
    super.key,
    required this.value,
    required this.onChanged,
    this.titlePreview = 'Judul Banner',
    this.bodyPreview = 'Deskripsi informasi pada banner...',
    this.category = 'announcement',
  });

  @override
  ConsumerState<BannerStyleEditor> createState() => _BannerStyleEditorState();
}

class _BannerStyleEditorState extends ConsumerState<BannerStyleEditor> {
  final GlobalKey _cropperKey = GlobalKey();
  final TextEditingController _hexController = TextEditingController();

  late String _backgroundType;
  late String _currentColor;
  late double _zoom;
  late bool _showDetail;
  late double _overlayOpacity;
  String? _imageUrl;
  File? _localFile;
  double _panX = 0.0;
  double _panY = 0.0;
  bool _isUploading = false;

  Offset _startFocal = Offset.zero;
  double _startPanX = 0.0;
  double _startPanY = 0.0;
  double _startZoom = 1.0;

  @override
  void initState() {
    super.initState();
    _backgroundType = widget.value?.backgroundType ?? 'color';
    _currentColor = widget.value?.backgroundColor ?? '#A970FF';
    _hexController.text = _currentColor;
    _imageUrl = widget.value?.image?.url;
    _zoom = widget.value?.image?.zoom ?? 1.0;
    _showDetail = widget.value?.showDetail ?? true;
    _overlayOpacity = widget.value?.overlayOpacity ?? 0.35;
    if (widget.value?.image?.crop != null) {
      _panX = widget.value!.image!.crop!.x;
      _panY = widget.value!.image!.crop!.y;
    }
  }

  @override
  void dispose() {
    _hexController.dispose();
    super.dispose();
  }

  CropRect get _currentCrop {
    final cropW = (1.0 / _zoom).clamp(0.01, 1.0);
    final cropH = (1.0 / _zoom).clamp(0.01, 1.0);
    final clampedX = _panX.clamp(0.0, 1.0 - cropW);
    final clampedY = _panY.clamp(0.0, 1.0 - cropH);
    return CropRect(
      x: clampedX,
      y: clampedY,
      width: cropW,
      height: cropH,
    );
  }

  void _notifyChange() {
    if (_backgroundType == 'color') {
      final grad = computeNeonGradientConfig(_currentColor);
      widget.onChanged(
        BannerStyleConfig(
          version: 1,
          backgroundType: 'color',
          backgroundColor: _currentColor,
          gradient: grad,
          image: null,
          overlayOpacity: _overlayOpacity,
          showDetail: _showDetail,
        ),
      );
    } else {
      final crop = _currentCrop;
      widget.onChanged(
        BannerStyleConfig(
          version: 1,
          backgroundType: 'image',
          backgroundColor: null,
          gradient: const GradientConfig(
            from: '#2A124D',
            to: '#130924',
            angle: 135,
            glow: true,
          ),
          image: ImageConfig(
            url: _imageUrl,
            crop: crop,
            focalPoint: FocalPoint(
              x: crop.x + crop.width / 2,
              y: crop.y + crop.height / 2,
            ),
            zoom: _zoom,
          ),
          overlayOpacity: _overlayOpacity,
          showDetail: _showDetail,
        ),
      );
    }
  }

  Future<void> _pickAndUploadImage() async {
    final picker = ImagePicker();
    final picked = await picker.pickImage(source: ImageSource.gallery);
    if (picked == null) return;

    final file = File(picked.path);
    final size = await file.length();
    if (size > 5 * 1024 * 1024) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Ukuran gambar maksimal 5 MB')),
        );
      }
      return;
    }

    setState(() {
      _localFile = file;
      _zoom = 1.0;
      _panX = 0.0;
      _panY = 0.0;
      _isUploading = true;
    });
    _notifyChange();

    try {
      final repo = ref.read(communityRepositoryProvider);
      final res = await repo.uploadCommunityImage(file, widget.category);
      final url = res['image_url'] as String?;
      if (url != null && url.isNotEmpty) {
        if (mounted) {
          setState(() {
            _imageUrl = url;
            _isUploading = false;
          });
          _notifyChange();
        }
      } else {
        if (mounted) {
          setState(() => _isUploading = false);
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Gagal mendapatkan URL gambar')),
          );
        }
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isUploading = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal mengunggah gambar: $e')),
        );
      }
    }
  }

  void _resetCrop() {
    setState(() {
      _zoom = 1.0;
      _panX = 0.0;
      _panY = 0.0;
    });
    _notifyChange();
  }

  void _nudge(double dx, double dy) {
    final cropW = (1.0 / _zoom).clamp(0.01, 1.0);
    final cropH = (1.0 / _zoom).clamp(0.01, 1.0);
    setState(() {
      _panX = (_panX + dx).clamp(0.0, 1.0 - cropW);
      _panY = (_panY + dy).clamp(0.0, 1.0 - cropH);
    });
    _notifyChange();
  }

  void _setZoom(double newZoom) {
    final clampedZoom = newZoom.clamp(1.0, 3.0);
    final oldCrop = _currentCrop;
    final focalX = oldCrop.x + oldCrop.width / 2;
    final focalY = oldCrop.y + oldCrop.height / 2;

    final newCropW = (1.0 / clampedZoom).clamp(0.01, 1.0);
    final newCropH = (1.0 / clampedZoom).clamp(0.01, 1.0);

    setState(() {
      _zoom = clampedZoom;
      _panX = (focalX - newCropW / 2).clamp(0.0, 1.0 - newCropW);
      _panY = (focalY - newCropH / 2).clamp(0.0, 1.0 - newCropH);
    });
    _notifyChange();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final hasImage = _localFile != null || (_imageUrl != null && _imageUrl!.isNotEmpty);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Kustomisasi Tampilan Background Banner',
          style: theme.textTheme.titleSmall?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 8),

        // 1. Mode Switcher
        Container(
          padding: const EdgeInsets.all(4),
          decoration: BoxDecoration(
            color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
            borderRadius: BorderRadius.circular(AppRadius.base),
            border: Border.all(
              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
            ),
          ),
          child: Row(
            children: [
              Expanded(
                child: InkWell(
                  onTap: () {
                    setState(() => _backgroundType = 'color');
                    _notifyChange();
                  },
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  child: Container(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    decoration: BoxDecoration(
                      color: _backgroundType == 'color'
                          ? (isDark ? AppColors.darkSurface : Colors.white)
                          : Colors.transparent,
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                      boxShadow: _backgroundType == 'color'
                          ? [
                              BoxShadow(
                                color: Colors.black.withValues(alpha: 0.08),
                                blurRadius: 4,
                              ),
                            ]
                          : null,
                    ),
                    alignment: Alignment.center,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.palette_outlined,
                          size: 16,
                          color: _backgroundType == 'color'
                              ? theme.colorScheme.primary
                              : theme.colorScheme.onSurfaceVariant,
                        ),
                        const SizedBox(width: 6),
                        Text(
                          'Warna Custom',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: _backgroundType == 'color'
                                ? theme.colorScheme.primary
                                : theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
              Expanded(
                child: InkWell(
                  onTap: () {
                    setState(() => _backgroundType = 'image');
                    _notifyChange();
                  },
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  child: Container(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    decoration: BoxDecoration(
                      color: _backgroundType == 'image'
                          ? (isDark ? AppColors.darkSurface : Colors.white)
                          : Colors.transparent,
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                      boxShadow: _backgroundType == 'image'
                          ? [
                              BoxShadow(
                                color: Colors.black.withValues(alpha: 0.08),
                                blurRadius: 4,
                              ),
                            ]
                          : null,
                    ),
                    alignment: Alignment.center,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.image_outlined,
                          size: 16,
                          color: _backgroundType == 'image'
                              ? theme.colorScheme.primary
                              : theme.colorScheme.onSurfaceVariant,
                        ),
                        const SizedBox(width: 6),
                        Text(
                          'Gambar (16:9)',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: _backgroundType == 'image'
                                ? theme.colorScheme.primary
                                : theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),

        // 2. Color Mode Controls
        if (_backgroundType == 'color') ...[
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
              borderRadius: BorderRadius.circular(AppRadius.base),
              border: Border.all(
                color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pilih Preset Warna Neon Glow:',
                  style: theme.textTheme.bodySmall?.copyWith(
                    fontWeight: FontWeight.w500,
                  ),
                ),
                const SizedBox(height: 10),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: neonColorPresets.map((p) {
                    final isSelected =
                        _currentColor.toUpperCase() == p['hex']!.toUpperCase();
                    final color = parseHexColor(p['hex']!);
                    return GestureDetector(
                      onTap: () {
                        setState(() {
                          _currentColor = p['hex']!;
                          _hexController.text = _currentColor;
                        });
                        _notifyChange();
                      },
                      child: Container(
                        width: 34,
                        height: 34,
                        decoration: BoxDecoration(
                          color: color,
                          borderRadius: BorderRadius.circular(8),
                          border: Border.all(
                            color: isSelected ? Colors.white : Colors.white24,
                            width: isSelected ? 2.5 : 1.0,
                          ),
                          boxShadow: isSelected
                              ? [
                                  BoxShadow(
                                    color: color.withValues(alpha: 0.6),
                                    blurRadius: 8,
                                    spreadRadius: 1,
                                  ),
                                ]
                              : null,
                        ),
                        child: isSelected
                            ? const Icon(Icons.check, size: 18, color: Colors.white)
                            : null,
                      ),
                    );
                  }).toList(),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Container(
                      width: 34,
                      height: 34,
                      decoration: BoxDecoration(
                        color: parseHexColor(_currentColor),
                        borderRadius: BorderRadius.circular(AppRadius.xs),
                        border: Border.all(color: Colors.white24),
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: TextField(
                        controller: _hexController,
                        decoration: const InputDecoration(
                          labelText: 'Kode Hex Warna (Custom)',
                          hintText: '#A970FF',
                          isDense: true,
                          contentPadding: EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                          border: OutlineInputBorder(),
                        ),
                        onChanged: (val) {
                          final clean = val.trim();
                          if (RegExp(r'^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$').hasMatch(clean)) {
                            final normalized = clean.startsWith('#') ? clean.toUpperCase() : '#${clean.toUpperCase()}';
                            setState(() => _currentColor = normalized);
                            _notifyChange();
                          }
                        },
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
        ],

        // 3. Image Mode Controls & Interactive Cropper
        if (_backgroundType == 'image') ...[
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
              borderRadius: BorderRadius.circular(AppRadius.base),
              border: Border.all(
                color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    ElevatedButton.icon(
                      onPressed: _isUploading ? null : _pickAndUploadImage,
                      icon: _isUploading
                          ? const SizedBox(
                              width: 14,
                              height: 14,
                              child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                            )
                          : const Icon(Icons.upload_file, size: 16),
                      label: Text(
                        hasImage ? 'Ganti Gambar' : 'Pilih Gambar (Galeri)',
                        style: const TextStyle(fontSize: 12),
                      ),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: theme.colorScheme.primary,
                        foregroundColor: Colors.white,
                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      ),
                    ),
                    if (hasImage) ...[
                      Row(
                        children: [
                          TextButton.icon(
                            onPressed: _resetCrop,
                            icon: const Icon(Icons.refresh, size: 14),
                            label: const Text('Reset', style: TextStyle(fontSize: 12)),
                          ),
                          IconButton(
                            onPressed: () {
                              setState(() {
                                _localFile = null;
                                _imageUrl = null;
                                _resetCrop();
                              });
                              _notifyChange();
                            },
                            icon: const Icon(Icons.delete_outline, size: 18, color: AppColors.danger),
                            tooltip: 'Hapus Gambar',
                          ),
                        ],
                      ),
                    ],
                  ],
                ),

                if (hasImage) ...[
                  const SizedBox(height: 12),
                  Text(
                    'Editor Crop 16:9 (Geser/Drag untuk reposisi, Pinch/Slider untuk zoom):',
                    style: theme.textTheme.bodySmall?.copyWith(fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 6),

                  // Interactive Cropper Box
                  AspectRatio(
                    key: _cropperKey,
                    aspectRatio: 16 / 9,
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: Stack(
                        children: [
                          // Base violet gradient
                          Container(
                            decoration: const BoxDecoration(
                              gradient: LinearGradient(
                                colors: [Color(0xFF2A124D), Color(0xFF130924)],
                                begin: Alignment.topLeft,
                                end: Alignment.bottomRight,
                              ),
                            ),
                          ),

                          // Crop image rendered with exact math
                          Positioned.fill(
                            child: BannerCropImage(
                              localFile: _localFile,
                              imageUrl: _imageUrl,
                              crop: _currentCrop,
                              zoom: _zoom,
                              alt: 'Banner Cropper Workspace',
                            ),
                          ),

                          // Rule of Thirds Crop Grid
                          Positioned.fill(
                            child: IgnorePointer(
                              child: CustomPaint(
                                painter: _RuleOfThirdsPainter(),
                              ),
                            ),
                          ),

                          // Visual boundary indicator
                          Positioned.fill(
                            child: IgnorePointer(
                              child: Container(
                                decoration: BoxDecoration(
                                  border: Border.all(color: Colors.white.withValues(alpha: 0.5), width: 1.5),
                                  borderRadius: BorderRadius.circular(AppRadius.base),
                                ),
                              ),
                            ),
                          ),

                          // Gesture detector for Pan & Pinch-to-zoom
                          Positioned.fill(
                            child: GestureDetector(
                              behavior: HitTestBehavior.opaque,
                              onScaleStart: (details) {
                                _startPanX = _panX;
                                _startPanY = _panY;
                                _startZoom = _zoom;
                                _startFocal = details.localFocalPoint;
                              },
                              onScaleUpdate: (details) {
                                final RenderBox? box = _cropperKey.currentContext?.findRenderObject() as RenderBox?;
                                final size = box?.size ?? const Size(320, 180);

                                double nextZoom = _zoom;
                                if (details.scale != 1.0) {
                                  nextZoom = (_startZoom * details.scale).clamp(1.0, 3.0);
                                }

                                final nextCropW = (1.0 / nextZoom).clamp(0.01, 1.0);
                                final nextCropH = (1.0 / nextZoom).clamp(0.01, 1.0);

                                final delta = details.localFocalPoint - _startFocal;
                                final normDeltaX = -delta.dx / (size.width / nextCropW);
                                final normDeltaY = -delta.dy / (size.height / nextCropH);

                                setState(() {
                                  _zoom = nextZoom;
                                  _panX = (_startPanX + normDeltaX).clamp(0.0, 1.0 - nextCropW);
                                  _panY = (_startPanY + normDeltaY).clamp(0.0, 1.0 - nextCropH);
                                });
                                _notifyChange();
                              },
                            ),
                          ),

                          // Top-right drag indicator hint pill
                          Positioned(
                            top: 8,
                            right: 8,
                            child: IgnorePointer(
                              child: Container(
                                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                                decoration: BoxDecoration(
                                  color: Colors.black.withValues(alpha: 0.65),
                                  borderRadius: BorderRadius.circular(AppRadius.xs),
                                ),
                                child: Text(
                                  'Drag / Pan Aktif • ${_zoom.toStringAsFixed(1)}x',
                                  style: const TextStyle(fontSize: 10, color: Colors.white, fontWeight: FontWeight.bold),
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),

                  const SizedBox(height: 10),

                  // Quick Zoom buttons & Nudge Arrows
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      // Zoom chips
                      Row(
                        children: [1.0, 1.5, 2.0, 3.0].map((z) {
                          final active = (_zoom - z).abs() < 0.05;
                          return Padding(
                            padding: const EdgeInsets.only(right: 6),
                            child: InkWell(
                              onTap: () => _setZoom(z),
                              borderRadius: BorderRadius.circular(4),
                              child: Container(
                                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                                decoration: BoxDecoration(
                                  color: active ? theme.colorScheme.primary : (isDark ? Colors.white12 : Colors.black12),
                                  borderRadius: BorderRadius.circular(4),
                                ),
                                child: Text(
                                  '${z.toStringAsFixed(1)}x',
                                  style: TextStyle(
                                    fontSize: 11,
                                    fontWeight: FontWeight.bold,
                                    color: active ? Colors.white : (isDark ? Colors.white70 : Colors.black87),
                                  ),
                                ),
                              ),
                            ),
                          );
                        }).toList(),
                      ),

                      // Nudge buttons for micro-adjustments
                      Row(
                        children: [
                          IconButton(
                            visualDensity: VisualDensity.compact,
                            icon: const Icon(Icons.arrow_left, size: 20),
                            onPressed: () => _nudge(-0.03, 0),
                            tooltip: 'Geser Kiri',
                          ),
                          IconButton(
                            visualDensity: VisualDensity.compact,
                            icon: const Icon(Icons.arrow_upward, size: 16),
                            onPressed: () => _nudge(0, -0.03),
                            tooltip: 'Geser Atas',
                          ),
                          IconButton(
                            visualDensity: VisualDensity.compact,
                            icon: const Icon(Icons.arrow_downward, size: 16),
                            onPressed: () => _nudge(0, 0.03),
                            tooltip: 'Geser Bawah',
                          ),
                          IconButton(
                            visualDensity: VisualDensity.compact,
                            icon: const Icon(Icons.arrow_right, size: 20),
                            onPressed: () => _nudge(0.03, 0),
                            tooltip: 'Geser Kanan',
                          ),
                        ],
                      ),
                    ],
                  ),

                  // Zoom Slider
                  Row(
                    children: [
                      const Text('Zoom: ', style: TextStyle(fontSize: 12)),
                      Expanded(
                        child: Slider(
                          value: _zoom,
                          min: 1.0,
                          max: 3.0,
                          divisions: 20,
                          label: '${_zoom.toStringAsFixed(1)}x',
                          onChanged: (val) => _setZoom(val),
                        ),
                      ),
                      Text(
                        '${_zoom.toStringAsFixed(1)}x',
                        style: const TextStyle(fontSize: 12, fontFamily: 'monospace'),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(height: 12),
        ],

        // 4. Show Detail Toggle
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
          decoration: BoxDecoration(
            color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
            borderRadius: BorderRadius.circular(AppRadius.base),
            border: Border.all(
              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
            ),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Tampilkan Detail Banner',
                      style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                    ),
                    Text(
                      'Sembunyikan teks detail jika gambar sudah lengkap',
                      style: TextStyle(
                        fontSize: 11,
                        color: isDark ? Colors.white60 : Colors.black54,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              Switch(
                value: _showDetail,
                activeColor: AppColors.success,
                onChanged: (val) {
                  setState(() => _showDetail = val);
                  _notifyChange();
                },
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // 5. Live 16:9 Preview
        Text(
          'Live Preview Hasil Banner (Rasio 16:9):',
          style: theme.textTheme.bodySmall?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 8),
        AspectRatio(
          aspectRatio: 16 / 9,
          child: ClipRRect(
            borderRadius: BorderRadius.circular(AppRadius.base),
            child: Stack(
              children: [
                // Base background
                if (_backgroundType == 'color') ...[
                  Positioned.fill(
                    child: BannerNeonGlowBackground(
                      color: parseHexColor(_currentColor),
                    ),
                  ),
                ] else ...[
                  // Image mode: base violet gradient for transparency support
                  Container(
                    decoration: const BoxDecoration(
                      gradient: LinearGradient(
                        colors: [Color(0xFF2A124D), Color(0xFF130924)],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                    ),
                  ),
                  if (hasImage)
                    Positioned.fill(
                      child: BannerCropImage(
                        localFile: _localFile,
                        imageUrl: _imageUrl,
                        crop: _currentCrop,
                        zoom: _zoom,
                        alt: 'Banner Preview',
                      ),
                    ),
                  // Dark bottom gradient overlay
                  Positioned.fill(
                    child: Container(
                      decoration: BoxDecoration(
                        gradient: LinearGradient(
                          colors: [
                            Colors.black.withValues(alpha: _overlayOpacity),
                            Colors.black.withValues(alpha: _overlayOpacity * 0.4),
                            Colors.transparent,
                          ],
                          begin: Alignment.bottomCenter,
                          end: Alignment.topCenter,
                          stops: const [0.0, 0.5, 1.0],
                        ),
                      ),
                    ),
                  ),
                ],

                // Foreground content
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      // Badge
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 8,
                          vertical: 4,
                        ),
                        decoration: BoxDecoration(
                          color: Colors.black.withValues(alpha: 0.4),
                          borderRadius: BorderRadius.circular(AppRadius.xs),
                          border: Border.all(color: Colors.white24),
                        ),
                        child: Text(
                          widget.category == 'activity' ? 'Kegiatan' : 'Pengumuman',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 10,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),

                      // Title & body
                      Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            widget.titlePreview.isEmpty
                                ? 'Judul Banner'
                                : widget.titlePreview,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 16,
                              fontWeight: FontWeight.bold,
                              shadows: [
                                Shadow(color: Colors.black87, blurRadius: 4),
                              ],
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          if (_showDetail && widget.bodyPreview.isNotEmpty) ...[
                            const SizedBox(height: 2),
                            Text(
                              widget.bodyPreview,
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 12,
                                shadows: [
                                  Shadow(color: Colors.black87, blurRadius: 3),
                                ],
                              ),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _RuleOfThirdsPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.white.withValues(alpha: 0.35)
      ..strokeWidth = 1.0
      ..style = PaintingStyle.stroke;

    final x1 = size.width / 3;
    final x2 = size.width * 2 / 3;
    final y1 = size.height / 3;
    final y2 = size.height * 2 / 3;

    canvas.drawLine(Offset(x1, 0), Offset(x1, size.height), paint);
    canvas.drawLine(Offset(x2, 0), Offset(x2, size.height), paint);
    canvas.drawLine(Offset(0, y1), Offset(size.width, y1), paint);
    canvas.drawLine(Offset(0, y2), Offset(size.width, y2), paint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

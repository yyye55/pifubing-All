import torch
import torch.nn as nn
import torch.optim as optim
import torch.nn.functional as F
from torchvision import transforms, models
from torch.utils.data import Dataset, DataLoader
import os
import time
from PIL import Image
from sklearn.model_selection import train_test_split
from torch.nn import TransformerEncoder, TransformerEncoderLayer

# ======================= 配置参数 =======================
DATA_ROOT = r"D:\学习资料\实验室\皮肤病\skin_disease_2.0\data\train"
NUM_CLASSES = 15
IMAGE_SIZE = 300
BATCH_SIZE = 16
ACCUM_STEPS = 4
MAX_EPOCHS = 100
NUM_WORKERS = 8
DEVICE = torch.device("cuda" if torch.cuda.is_available() else "cpu")
PRETRAINED_WEIGHTS = "B3.pth"


# ======================= Focal Loss =======================
class BalancedFocalLoss(nn.Module):
    def __init__(self, class_counts, gamma=2.0, alpha_ratio=0.8):
        super().__init__()
        weights = 1.0 / (class_counts + 1e-6)
        self.alpha = torch.pow(weights / weights.max(), alpha_ratio).to(DEVICE)
        self.gamma = gamma

    def forward(self, inputs, targets):
        ce_loss = F.cross_entropy(inputs, targets, reduction='none')
        pt = torch.exp(-ce_loss)
        loss = self.alpha[targets] * (1 - pt) ** self.gamma * ce_loss
        return loss.mean()


# ======================= 数据集类 =======================
class SkinDataset(Dataset):
    def __init__(self, root_dir, transform=None):
        self.classes = sorted(os.listdir(root_dir))
        self.class_to_idx = {cls: i for i, cls in enumerate(self.classes)}
        self.samples = []
        self.transform = transform
        self.class_counts = []

        for cls in self.classes:
            cls_dir = os.path.join(root_dir, cls)
            if os.path.isdir(cls_dir):
                count = 0
                for img_name in os.listdir(cls_dir):
                    img_path = os.path.join(cls_dir, img_name)
                    if os.path.isfile(img_path):
                        self.samples.append((img_path, self.class_to_idx[cls]))
                        count += 1
                self.class_counts.append(count)

    def __len__(self):
        return len(self.samples)

    def __getitem__(self, idx):
        img_path, label = self.samples[idx]
        image = Image.open(img_path).convert('RGB')
        if self.transform:
            image = self.transform(image)
        return image, label


# ======================= 增强的数据预处理 =======================
train_transform = transforms.Compose([
    transforms.RandomResizedCrop(IMAGE_SIZE, scale=(0.6, 1.0), ratio=(0.8, 1.2)),
    transforms.RandomHorizontalFlip(),
    transforms.RandomVerticalFlip(),
    transforms.ColorJitter(brightness=0.2, contrast=0.2, saturation=0.2),
    transforms.RandomRotation(30),
    transforms.ToTensor(),
    transforms.Normalize([0.485, 0.456, 0.406], [0.229, 0.224, 0.225]),
    transforms.RandomErasing(p=0.3, scale=(0.02, 0.1))
])


# ======================= 混合模型定义 =======================
class B3Transformer(nn.Module):
    def __init__(self, num_classes):
        super().__init__()

        # EfficientNetB3主干
        self.cnn = models.efficientnet_b3(weights=None)
        in_features = self.cnn.classifier[1].in_features

        # 加载预训练权重
        pretrained_dict = torch.load(PRETRAINED_WEIGHTS)
        # 去除键名的 'backbone.' 前缀
        new_pretrained_dict = {k.replace('backbone.', ''): v for k, v in pretrained_dict.items()}
        # 获取模型的状态字典
        model_dict = self.cnn.state_dict()
        # 过滤掉不匹配的键和分类头的参数
        new_pretrained_dict = {k: v for k, v in new_pretrained_dict.items() if
                               k in model_dict and 'classifier' not in k}
        # 更新模型的状态字典
        model_dict.update(new_pretrained_dict)
        # 加载更新后的状态字典
        self.cnn.load_state_dict(model_dict)

        # 冻结前4个MBConv块
        for param in self.cnn.features[:5].parameters():
            param.requires_grad = False

        # 修改分类头以适应指定的类别数
        self.cnn.classifier[1] = nn.Linear(in_features, num_classes)

        # Transformer分支
        self.patch_embed = nn.Sequential(
            nn.Conv2d(3, 256, kernel_size=16, stride=16),
            nn.BatchNorm2d(256),
            nn.GELU()
        )

        encoder_layer = TransformerEncoderLayer(
            d_model=256,
            nhead=8,
            dim_feedforward=1024,
            dropout=0.1,
            activation='gelu',
            batch_first=True
        )
        self.transformer = TransformerEncoder(encoder_layer, num_layers=3)

        # 特征融合模块
        self.fusion = nn.Sequential(
            nn.Conv2d(1536 + 256, 1024, 1),
            nn.BatchNorm2d(1024),
            nn.SiLU(),
            nn.Dropout2d(0.3)
        )

        # 分类头
        self.classifier = nn.Sequential(
            nn.AdaptiveAvgPool2d(1),
            nn.Flatten(),
            nn.Linear(1024, 512),
            nn.BatchNorm1d(512),
            nn.SiLU(),
            nn.Dropout(0.5),
            nn.Linear(512, num_classes)
        )

    def forward(self, x):
        # CNN特征提取
        cnn_feat = self.cnn.features(x)

        # Transformer处理
        patches = self.patch_embed(x)  # [B, 256, H/16, W/16]
        B, C, H, W = patches.shape
        patches = patches.view(B, C, -1).permute(0, 2, 1)
        trans_feat = self.transformer(patches)
        trans_feat = trans_feat.mean(dim=1)[:, :, None, None]
        trans_feat = F.interpolate(trans_feat, size=cnn_feat.shape[2:])

        # 特征融合
        fused = torch.cat([cnn_feat, trans_feat], dim=1)
        fused = self.fusion(fused)
        return self.classifier(fused)


# ======================= 训练流程 =======================
def main():
    # 准备数据集
    full_dataset = SkinDataset(DATA_ROOT, transform=train_transform)

    # 划分数据集
    train_indices, val_indices = train_test_split(
        range(len(full_dataset)),
        test_size=0.2,
        stratify=[label for (_, label) in full_dataset.samples]
    )

    train_loader = DataLoader(
        torch.utils.data.Subset(full_dataset, train_indices),
        batch_size=BATCH_SIZE,
        shuffle=True,
        num_workers=NUM_WORKERS,
        pin_memory=True,
        drop_last=True
    )

    val_loader = DataLoader(
        torch.utils.data.Subset(full_dataset, val_indices),
        batch_size=BATCH_SIZE * 2,
        num_workers=NUM_WORKERS,
        pin_memory=True
    )

    # 初始化模型
    model = B3Transformer(NUM_CLASSES).to(DEVICE)

    # 损失函数
    class_counts = torch.tensor(full_dataset.class_counts, dtype=torch.float32)
    criterion = BalancedFocalLoss(class_counts)

    # 优化器配置
    optimizer = optim.AdamW([
        {'params': model.cnn.parameters(), 'lr': 1e-5},
        {'params': model.transformer.parameters()},
        {'params': model.fusion.parameters()},
        {'params': model.classifier.parameters()}
    ], lr=3e-4, weight_decay=1e-4)

    # 学习率调度
    scheduler = optim.lr_scheduler.OneCycleLR(
        optimizer,
        max_lr=3e-4,
        total_steps=MAX_EPOCHS * len(train_loader),
        pct_start=0.3,
        anneal_strategy='cos'
    )

    # 混合精度训练
    scaler = torch.amp.GradScaler('cuda')

    # 训练循环
    best_val_acc = 0.0
    start_time = time.time()
    patience = 15  # 早停机制的耐心值

    for epoch in range(MAX_EPOCHS):
        model.train()
        train_loss = 0.0

        for step, (images, labels) in enumerate(train_loader):
            images = images.to(DEVICE, non_blocking=True)
            labels = labels.to(DEVICE, non_blocking=True)

            with torch.amp.autocast('cuda'):
                outputs = model(images)
                loss = criterion(outputs, labels) / ACCUM_STEPS

            scaler.scale(loss).backward()

            if (step + 1) % ACCUM_STEPS == 0:
                scaler.unscale_(optimizer)
                torch.nn.utils.clip_grad_norm_(model.parameters(), 5.0)
                scaler.step(optimizer)
                scaler.update()
                optimizer.zero_grad()
                scheduler.step()

            train_loss += loss.item() * ACCUM_STEPS

        # 验证阶段
        model.eval()
        correct = 0
        total = 0
        with torch.no_grad():
            for images, labels in val_loader:
                images = images.to(DEVICE)
                labels = labels.to(DEVICE)
                outputs = model(images)
                _, predicted = torch.max(outputs, 1)
                total += labels.size(0)
                correct += (predicted == labels).sum().item()

        val_acc = 100 * correct / total
        avg_loss = train_loss / len(train_loader)
        print(f"Epoch [{epoch + 1}/{MAX_EPOCHS}] | Loss: {avg_loss:.4f} | "
              f"Val Acc: {val_acc:.2f}% | LR: {scheduler.get_last_lr()[0]:.2e}")

        # 早停机制
        if val_acc > best_val_acc:
            best_val_acc = val_acc
            torch.save(model.state_dict(), "B3_Transformer.pth")
            patience = 15
        else:
            patience -= 1
            if patience <= 0:
                print(f"Early stopping at epoch {epoch + 1}")
                break

    print(f"\nTraining completed in {(time.time() - start_time) / 3600:.1f} hours")
    print(f"Best validation accuracy: {best_val_acc:.2f}%")


if __name__ == '__main__':
    torch.multiprocessing.freeze_support()
    main()